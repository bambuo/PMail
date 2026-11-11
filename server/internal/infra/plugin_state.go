package infra

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PluginState 是 WASM 插件的审批与运行状态，持久化在 plugins/wasm/state.json。
type PluginState struct {
	Status              string   `json:"status"`
	Version             string   `json:"version"`
	ABI                 string   `json:"abi"`
	ManifestSHA256      string   `json:"manifestSha256"`
	WasmSHA256          string   `json:"wasmSha256"`
	GrantedHooks        []string `json:"grantedHooks"`
	GrantedCapabilities []string `json:"grantedCapabilities"`
	ApprovedAt          string   `json:"approvedAt"`
	LastUsedAt          string   `json:"lastUsedAt"`
	CallCount           int64    `json:"callCount"`
	ErrorCount          int64    `json:"errorCount"`
	LastError           string   `json:"lastError"`
}

type pluginStateFile struct {
	Version int                    `json:"version"`
	Plugins map[string]PluginState `json:"plugins"`
}

// PluginStateStore 负责状态文件的并发读写与原子落盘。
type PluginStateStore struct {
	path string
	mu   sync.RWMutex
	data pluginStateFile
}

// NewPluginStateStore 读取状态文件；文件损坏时改名为 .corrupt 后以空状态启动，
// 同时返回错误供调用方记录。
func NewPluginStateStore(path string) (*PluginStateStore, error) {
	s := &PluginStateStore{
		path: path,
		data: pluginStateFile{Version: 1, Plugins: map[string]PluginState{}},
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	var data pluginStateFile
	if err := json.Unmarshal(raw, &data); err != nil {
		corrupt := fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix())
		_ = os.Rename(path, corrupt)
		return s, fmt.Errorf("state file corrupt, moved to %s: %w", corrupt, err)
	}
	if data.Plugins == nil {
		data.Plugins = map[string]PluginState{}
	}
	if data.Version == 0 {
		data.Version = 1
	}
	s.data = data
	return s, nil
}

func (s *PluginStateStore) Get(id string) (PluginState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.data.Plugins[id]
	return st, ok
}

func (s *PluginStateStore) Put(id string, st PluginState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Plugins[id] = st
	return s.saveLocked()
}

// Upsert 在锁内读取最新状态并允许调用方只修改需要的字段，避免与计数落盘互相覆盖。
// 回调返回错误时不写入文件。
func (s *PluginStateStore) Upsert(id string, fn func(st *PluginState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.data.Plugins[id]
	if err := fn(&st); err != nil {
		return err
	}
	s.data.Plugins[id] = st
	return s.saveLocked()
}

func (s *PluginStateStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data.Plugins, id)
	return s.saveLocked()
}

func (s *PluginStateStore) Snapshot() map[string]PluginState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]PluginState, len(s.data.Plugins))
	for id, st := range s.data.Plugins {
		st.GrantedHooks = append([]string(nil), st.GrantedHooks...)
		st.GrantedCapabilities = append([]string(nil), st.GrantedCapabilities...)
		out[id] = st
	}
	return out
}

func (s *PluginStateStore) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path, raw, 0o600)
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		// Windows 上 rename 不能覆盖已有文件，退化为删除后重命名。
		if removeErr := os.Remove(path); removeErr == nil {
			if renameErr := os.Rename(tmpName, path); renameErr == nil {
				return nil
			}
		}
		return err
	}
	return nil
}
