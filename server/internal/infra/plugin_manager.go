package infra

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/hooks/wasmabi"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

type runningPlugin struct {
	hook     *WasmHook
	exec     *wasmExecutor
	counters *pluginCounters
}

type pluginManager struct {
	root          string
	serverVersion string
	store         *PluginStateStore

	opMu sync.Mutex
	mu   sync.RWMutex

	artifacts map[string]*wasmArtifact
	running   map[string]*runningPlugin
	loadErrs  map[string]string

	stopCh   chan struct{}
	stopOnce sync.Once
}

var wasmManager *pluginManager

func newPluginManager(root, serverVersion string) *pluginManager {
	return &pluginManager{
		root:          root,
		serverVersion: serverVersion,
		artifacts:     map[string]*wasmArtifact{},
		running:       map[string]*runningPlugin{},
		loadErrs:      map[string]string{},
		stopCh:        make(chan struct{}),
	}
}

func (m *pluginManager) init(ctx context.Context) error {
	store, err := NewPluginStateStore(filepath.Join(m.root, "state.json"))
	if err != nil {
		log.Errorf("wasm plugin state load failed: %v", err)
	}
	m.store = store
	if err := m.rescanLocked(); err != nil {
		return err
	}
	states := store.Snapshot()
	for id := range states {
		if _, ok := m.artifacts[id]; !ok {
			st := states[id]
			st.Status = string(plugin.StatusError)
			st.LastError = "plugin directory not found"
			_ = store.Put(id, st)
		}
	}
	for id, art := range m.artifacts {
		st, ok := states[id]
		if !ok || st.ManifestSHA256 != art.ManifestSHA || st.WasmSHA256 != art.WasmSHA {
			st = PluginState{
				Status:         string(plugin.StatusPending),
				Version:        art.Manifest.Version,
				ABI:            art.Manifest.ABI,
				ManifestSHA256: art.ManifestSHA,
				WasmSHA256:     art.WasmSHA,
			}
			_ = store.Put(id, st)
		}
		if st.Status == string(plugin.StatusEnabled) {
			if err := m.load(ctx, id, st); err != nil {
				st.Status = string(plugin.StatusError)
				st.LastError = err.Error()
				_ = store.Put(id, st)
				log.Errorf("[%s] wasm plugin load failed: %v", id, err)
			}
		}
	}
	go m.flushLoop()
	return nil
}

func (m *pluginManager) flushLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.flushCounters()
		case <-m.stopCh:
			return
		}
	}
}

func (m *pluginManager) flushCounters() {
	m.mu.RLock()
	snapshot := make(map[string]*runningPlugin, len(m.running))
	for id, r := range m.running {
		snapshot[id] = r
	}
	m.mu.RUnlock()
	for id, r := range snapshot {
		call, errCount, lastErr, lastUsed := r.counters.snapshot()
		err := m.store.Upsert(id, func(st *PluginState) error {
			st.CallCount = call
			st.ErrorCount = errCount
			if lastErr != "" {
				st.LastError = lastErr
			}
			if lastUsed != "" {
				st.LastUsedAt = lastUsed
			}
			return nil
		})
		if err != nil {
			log.Errorf("[%s] persist plugin counters failed: %v", id, err)
		}
	}
}

func (m *pluginManager) rescanLocked() error {
	arts, errs := scanWasmPlugins(m.root)
	if len(errs) > 0 {
		for _, err := range errs {
			log.Errorf("wasm plugin scan: %v", err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.artifacts = map[string]*wasmArtifact{}
	for _, art := range arts {
		m.artifacts[art.Manifest.ID] = art
	}
	return nil
}

func (m *pluginManager) load(ctx context.Context, id string, st PluginState) error {
	art, ok := m.artifacts[id]
	if !ok {
		return plugin.ErrNotFound
	}
	grantedHooks := map[string]bool{}
	for _, h := range st.GrantedHooks {
		if !allowedHooks[h] || !contains(art.Manifest.Hooks, h) {
			return fmt.Errorf("granted hook %q is not declared in manifest", h)
		}
		grantedHooks[h] = true
	}
	grantedCaps := map[string]bool{}
	for _, c := range st.GrantedCapabilities {
		if !allowedCaps[c] || !contains(art.Manifest.Capabilities, c) {
			return fmt.Errorf("granted capability %q is not declared in manifest", c)
		}
		grantedCaps[c] = true
	}
	if _, exists := PluginGetHook(id); exists {
		return plugin.ErrNameConflict
	}
	exec, err := newWasmExecutor(ctx, art, grantedCaps)
	if err != nil {
		return fmt.Errorf("instantiate: %w", err)
	}
	hook := &WasmHook{
		id:            id,
		manifest:      art.Manifest,
		grantedHooks:  grantedHooks,
		grantedCaps:   grantedCaps,
		exec:          exec,
		serverVersion: m.serverVersion,
		counters:      &pluginCounters{},
	}
	if st.CallCount > 0 {
		hook.counters.callCount.Store(st.CallCount)
		hook.counters.errorCount.Store(st.ErrorCount)
	}
	if err := handshake(ctx, hook); err != nil {
		_ = exec.Close(context.Background())
		return fmt.Errorf("handshake: %w", err)
	}
	PluginRegisterHook(id, hook)
	m.mu.Lock()
	m.running[id] = &runningPlugin{hook: hook, exec: exec, counters: hook.counters}
	delete(m.loadErrs, id)
	m.mu.Unlock()
	return nil
}

func handshake(ctx context.Context, hook *WasmHook) error {
	body, err := json.Marshal(&hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "GetName",
		ServerVersion: hook.serverVersion,
	})
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, time.Duration(hook.manifest.Limits.TimeoutMS)*time.Millisecond)
	defer cancel()
	_, err = hook.exec.Call(callCtx, wasmabi.HookGetName, body)
	return err
}

func (m *pluginManager) unload(ctx context.Context, id string) {
	m.mu.Lock()
	r := m.running[id]
	delete(m.running, id)
	m.mu.Unlock()
	if r == nil {
		return
	}
	PluginRemoveHook(id)
	r.hook.drain(hookCallDrain)
	_ = r.exec.Close(ctx)
}

func (m *pluginManager) list() []plugin.Info {
	m.mu.RLock()
	arts := make(map[string]*wasmArtifact, len(m.artifacts))
	for id, art := range m.artifacts {
		arts[id] = art
	}
	running := make(map[string]*runningPlugin, len(m.running))
	for id, r := range m.running {
		running[id] = r
	}
	m.mu.RUnlock()

	states := m.store.Snapshot()
	ids := map[string]bool{}
	for id := range arts {
		ids[id] = true
	}
	for id := range states {
		ids[id] = true
	}
	out := make([]plugin.Info, 0, len(ids))
	for id := range ids {
		st := states[id]
		info := plugin.Info{
			ID:                  id,
			Type:                "wasm",
			Status:              plugin.Status(st.Status),
			Version:             st.Version,
			ABI:                 st.ABI,
			GrantedHooks:        append([]string{}, st.GrantedHooks...),
			GrantedCapabilities: append([]string{}, st.GrantedCapabilities...),
			LastError:           st.LastError,
			CallCount:           st.CallCount,
			ErrorCount:          st.ErrorCount,
			LastUsedAt:          st.LastUsedAt,
		}
		if info.Status == "" {
			info.Status = plugin.StatusPending
		}
		if art, ok := arts[id]; ok {
			info.Name = art.Manifest.Name
			info.Version = art.Manifest.Version
			info.ABI = art.Manifest.ABI
			info.Hooks = append([]string{}, art.Manifest.Hooks...)
			info.Capabilities = append([]string{}, art.Manifest.Capabilities...)
		}
		if r, ok := running[id]; ok {
			call, errCount, lastErr, lastUsed := r.counters.snapshot()
			info.CallCount = call
			info.ErrorCount = errCount
			if lastErr != "" {
				info.LastError = lastErr
			}
			if lastUsed != "" {
				info.LastUsedAt = lastUsed
			}
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (m *pluginManager) artifact(id string) (*wasmArtifact, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	art, ok := m.artifacts[id]
	return art, ok
}

func (m *pluginManager) approve(id string, hooks, capabilities []string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	art, ok := m.artifact(id)
	if !ok {
		return plugin.ErrNotFound
	}
	st, ok := m.store.Get(id)
	if !ok {
		st = PluginState{
			Status:         string(plugin.StatusPending),
			Version:        art.Manifest.Version,
			ABI:            art.Manifest.ABI,
			ManifestSHA256: art.ManifestSHA,
			WasmSHA256:     art.WasmSHA,
		}
	}
	if st.Status == string(plugin.StatusEnabled) {
		return plugin.ErrInvalidState
	}
	for _, h := range hooks {
		if !contains(art.Manifest.Hooks, h) {
			return fmt.Errorf("%w: hook %s", plugin.ErrInvalidGrant, h)
		}
	}
	for _, c := range capabilities {
		if !contains(art.Manifest.Capabilities, c) {
			return fmt.Errorf("%w: capability %s", plugin.ErrInvalidGrant, c)
		}
	}
	st.Status = string(plugin.StatusApproved)
	st.GrantedHooks = dedupe(hooks)
	st.GrantedCapabilities = dedupe(capabilities)
	st.ApprovedAt = time.Now().Format(time.RFC3339)
	st.Version = art.Manifest.Version
	st.ABI = art.Manifest.ABI
	st.ManifestSHA256 = art.ManifestSHA
	st.WasmSHA256 = art.WasmSHA
	st.LastError = ""
	return m.store.Upsert(id, func(cur *PluginState) error {
		cur.Status = st.Status
		cur.GrantedHooks = st.GrantedHooks
		cur.GrantedCapabilities = st.GrantedCapabilities
		cur.ApprovedAt = st.ApprovedAt
		cur.Version = st.Version
		cur.ABI = st.ABI
		cur.ManifestSHA256 = st.ManifestSHA256
		cur.WasmSHA256 = st.WasmSHA256
		cur.LastError = ""
		return nil
	})
}

func (m *pluginManager) enable(ctx context.Context, id string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	st, ok := m.store.Get(id)
	if !ok {
		return plugin.ErrNotFound
	}
	if st.Status != string(plugin.StatusApproved) && st.Status != string(plugin.StatusDisabled) && st.Status != string(plugin.StatusError) {
		return plugin.ErrInvalidState
	}
	if _, ok := m.artifact(id); !ok {
		return plugin.ErrNotFound
	}
	if err := m.load(ctx, id, st); err != nil {
		_ = m.store.Upsert(id, func(cur *PluginState) error {
			cur.Status = string(plugin.StatusError)
			cur.LastError = err.Error()
			return nil
		})
		return err
	}
	return m.store.Upsert(id, func(cur *PluginState) error {
		cur.Status = string(plugin.StatusEnabled)
		cur.LastError = ""
		return nil
	})
}

func (m *pluginManager) disable(ctx context.Context, id string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	st, ok := m.store.Get(id)
	if !ok {
		return plugin.ErrNotFound
	}
	if st.Status != string(plugin.StatusEnabled) {
		return plugin.ErrInvalidState
	}
	m.flushOne(id)
	m.unload(ctx, id)
	return m.store.Upsert(id, func(st *PluginState) error {
		st.Status = string(plugin.StatusDisabled)
		return nil
	})
}

func (m *pluginManager) reject(ctx context.Context, id string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	if _, ok := m.store.Get(id); !ok {
		return plugin.ErrNotFound
	}
	m.unload(ctx, id)
	return m.store.Upsert(id, func(cur *PluginState) error {
		cur.Status = string(plugin.StatusRejected)
		cur.GrantedHooks = nil
		cur.GrantedCapabilities = nil
		cur.LastError = ""
		return nil
	})
}

func (m *pluginManager) reload(ctx context.Context, id string) error {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	st, ok := m.store.Get(id)
	if !ok {
		return plugin.ErrNotFound
	}
	if st.Status != string(plugin.StatusEnabled) && st.Status != string(plugin.StatusError) {
		return plugin.ErrInvalidState
	}
	m.mu.RLock()
	old := m.running[id]
	m.mu.RUnlock()
	if old == nil {
		if st.Status != string(plugin.StatusError) {
			return plugin.ErrInvalidState
		}
		if err := m.load(ctx, id, st); err != nil {
			st.LastError = err.Error()
			_ = m.store.Put(id, st)
			return err
		}
		st.Status = string(plugin.StatusEnabled)
		st.LastError = ""
		return m.store.Put(id, st)
	}
	art, err := loadArtifact(filepath.Join(m.root, id))
	if err != nil {
		return err
	}
	for _, h := range st.GrantedHooks {
		if !contains(art.Manifest.Hooks, h) {
			return fmt.Errorf("%w: hook %s removed", plugin.ErrInvalidGrant, h)
		}
	}
	for _, c := range st.GrantedCapabilities {
		if !contains(art.Manifest.Capabilities, c) {
			return fmt.Errorf("%w: capability %s removed", plugin.ErrInvalidGrant, c)
		}
	}
	exec, err := newWasmExecutor(ctx, art, capsToMap(st.GrantedCapabilities))
	if err != nil {
		return err
	}
	hook := &WasmHook{
		id:            id,
		manifest:      art.Manifest,
		grantedHooks:  hooksToMap(st.GrantedHooks),
		grantedCaps:   capsToMap(st.GrantedCapabilities),
		exec:          exec,
		serverVersion: m.serverVersion,
		counters:      old.counters,
	}
	if err := handshake(ctx, hook); err != nil {
		_ = exec.Close(context.Background())
		return err
	}
	PluginRegisterHook(id, hook)
	m.mu.Lock()
	m.artifacts[id] = art
	m.running[id] = &runningPlugin{hook: hook, exec: exec, counters: old.counters}
	m.mu.Unlock()
	old.hook.drain(hookCallDrain)
	_ = old.exec.Close(ctx)
	return m.store.Upsert(id, func(cur *PluginState) error {
		cur.Version = art.Manifest.Version
		cur.ABI = art.Manifest.ABI
		cur.ManifestSHA256 = art.ManifestSHA
		cur.WasmSHA256 = art.WasmSHA
		cur.LastError = ""
		return nil
	})
}

func (m *pluginManager) flushOne(id string) {
	m.mu.RLock()
	r := m.running[id]
	m.mu.RUnlock()
	if r == nil {
		return
	}
	call, errCount, lastErr, lastUsed := r.counters.snapshot()
	_ = m.store.Upsert(id, func(st *PluginState) error {
		st.CallCount = call
		st.ErrorCount = errCount
		if lastErr != "" {
			st.LastError = lastErr
		}
		if lastUsed != "" {
			st.LastUsedAt = lastUsed
		}
		return nil
	})
}

func (m *pluginManager) stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.mu.RLock()
	ids := make([]string, 0, len(m.running))
	for id := range m.running {
		ids = append(ids, id)
	}
	m.mu.RUnlock()
	for _, id := range ids {
		m.flushOne(id)
		m.unload(context.Background(), id)
	}
}

func loadArtifact(dir string) (*wasmArtifact, error) {
	m, raw, err := LoadWasmManifest(dir)
	if err != nil {
		return nil, err
	}
	if m.ID != filepath.Base(dir) {
		return nil, fmt.Errorf("id %q must equal directory name", m.ID)
	}
	entry, err := m.EntryPath(dir)
	if err != nil {
		return nil, err
	}
	wasm, err := os.ReadFile(entry)
	if err != nil {
		return nil, err
	}
	return &wasmArtifact{
		Dir:         dir,
		Manifest:    m,
		ManifestSHA: sha256Hex(raw),
		WasmSHA:     sha256Hex(wasm),
		Wasm:        wasm,
	}, nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func dedupe(list []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func hooksToMap(list []string) map[string]bool {
	out := map[string]bool{}
	for _, v := range list {
		out[v] = true
	}
	return out
}

func capsToMap(list []string) map[string]bool { return hooksToMap(list) }

// PluginWasmInit 在配置开启时初始化 WASM 插件管理器。
func PluginWasmInit(serverVersion string) {
	if config.Instance == nil || config.Instance.PluginWasmEnabled != 1 {
		return
	}
	root := filepath.Join(config.ROOT_PATH, "plugins", "wasm")
	m := newPluginManager(root, serverVersion)
	if err := m.init(context.Background()); err != nil {
		log.Errorf("wasm plugin manager init failed: %v", err)
	}
	wasmManager = m
}

func PluginWasmStop() {
	if wasmManager == nil {
		return
	}
	wasmManager.stop()
	wasmManager = nil
}

func PluginManageList() []plugin.Info {
	if wasmManager == nil {
		return nil
	}
	return wasmManager.list()
}

func PluginManageApprove(ctx *pcontext.Context, id string, hooks, capabilities []string) error {
	if wasmManager == nil {
		return plugin.ErrDisabled
	}
	return wasmManager.approve(id, hooks, capabilities)
}

func PluginManageEnable(ctx *pcontext.Context, id string) error {
	if wasmManager == nil {
		return plugin.ErrDisabled
	}
	return wasmManager.enable(stdCtx(ctx), id)
}

func PluginManageDisable(ctx *pcontext.Context, id string) error {
	if wasmManager == nil {
		return plugin.ErrDisabled
	}
	return wasmManager.disable(stdCtx(ctx), id)
}

func PluginManageReject(ctx *pcontext.Context, id string) error {
	if wasmManager == nil {
		return plugin.ErrDisabled
	}
	return wasmManager.reject(stdCtx(ctx), id)
}

func PluginManageReload(ctx *pcontext.Context, id string) error {
	if wasmManager == nil {
		return plugin.ErrDisabled
	}
	return wasmManager.reload(stdCtx(ctx), id)
}

func stdCtx(ctx *pcontext.Context) context.Context {
	if ctx != nil && ctx.Context != nil {
		return ctx.Context
	}
	return context.Background()
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
