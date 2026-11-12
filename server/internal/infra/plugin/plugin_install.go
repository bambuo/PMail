package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Jinnrry/pmail/dto/plugin"
)

const (
	maxPluginPackageBytes  = 32 << 20
	maxPluginUnpackedBytes = 64 << 20
	maxPluginPackageFiles  = 16
)

// install 接受一个 zip 包，包根目录必须包含 manifest.json 与 manifest 声明的入口 wasm。
func (m *pluginManager) install(data []byte) (plugin.Info, error) {
	if m == nil || m.store == nil {
		return plugin.Info{}, plugin.ErrDisabled
	}
	if len(data) == 0 || len(data) > maxPluginPackageBytes {
		return plugin.Info{}, fmt.Errorf("plugin package must be between 1 byte and %d bytes", maxPluginPackageBytes)
	}
	files, err := readPluginZip(data)
	if err != nil {
		return plugin.Info{}, err
	}
	manifestRaw, ok := files["manifest.json"]
	if !ok {
		return plugin.Info{}, errors.New("plugin package must contain manifest.json at its root")
	}
	var manifest WasmManifest
	dec := json.NewDecoder(bytes.NewReader(manifestRaw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return plugin.Info{}, fmt.Errorf("parse manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return plugin.Info{}, err
	}
	entry := manifest.Entry
	if entry == "" {
		entry = "plugin.wasm"
	}
	if _, ok := files[entry]; !ok {
		return plugin.Info{}, fmt.Errorf("plugin package must contain entry %q", entry)
	}
	if len(files) != 2 {
		return plugin.Info{}, errors.New("plugin package must contain only manifest.json and the entry wasm")
	}

	id := manifest.ID
	target := filepath.Join(m.root, id)

	m.opMu.Lock()
	defer m.opMu.Unlock()
	if _, ok := m.artifact(id); ok {
		return plugin.Info{}, fmt.Errorf("plugin %q is already installed", id)
	}
	if _, err := os.Lstat(target); err == nil {
		return plugin.Info{}, fmt.Errorf("plugin %q is already installed", id)
	} else if !errors.Is(err, os.ErrNotExist) {
		return plugin.Info{}, err
	}
	if err := os.MkdirAll(m.root, 0o755); err != nil {
		return plugin.Info{}, fmt.Errorf("create plugin directory: %w", err)
	}
	tmp, err := os.MkdirTemp(m.root, ".install-")
	if err != nil {
		return plugin.Info{}, fmt.Errorf("create plugin staging directory: %w", err)
	}
	defer os.RemoveAll(tmp)
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(tmp, name), body, 0o600); err != nil {
			return plugin.Info{}, fmt.Errorf("write plugin file %s: %w", name, err)
		}
	}
	if _, _, err := LoadWasmManifest(tmp); err != nil {
		return plugin.Info{}, err
	}
	if err := os.Rename(tmp, target); err != nil {
		return plugin.Info{}, fmt.Errorf("install plugin: %w", err)
	}
	if err := m.rescanLocked(); err != nil {
		return plugin.Info{}, err
	}
	art, ok := m.artifact(id)
	if !ok {
		return plugin.Info{}, fmt.Errorf("plugin %q was not discovered after install", id)
	}
	st := PluginState{
		Status:         string(plugin.StatusPending),
		Version:        art.Manifest.Version,
		ABI:            art.Manifest.ABI,
		ManifestSHA256: art.ManifestSHA,
		WasmSHA256:     art.WasmSHA,
	}
	if err := m.store.Put(id, st); err != nil {
		return plugin.Info{}, fmt.Errorf("persist plugin state: %w", err)
	}
	for _, info := range m.list() {
		if info.ID == id {
			return info, nil
		}
	}
	return plugin.Info{ID: id, Name: art.Manifest.Name, Type: "wasm", Version: art.Manifest.Version, ABI: art.Manifest.ABI, Status: plugin.StatusPending}, nil
}

func readPluginZip(data []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open plugin package: %w", err)
	}
	files := make(map[string][]byte)
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			return nil, errors.New("plugin package must not contain directories")
		}
		name := f.Name
		if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
			return nil, fmt.Errorf("invalid plugin file name %q", name)
		}
		if f.FileInfo().Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("plugin file %q must not be a symlink", name)
		}
		if len(files) >= maxPluginPackageFiles {
			return nil, fmt.Errorf("plugin package must contain at most %d files", maxPluginPackageFiles)
		}
		if f.UncompressedSize64 > maxPluginUnpackedBytes {
			return nil, fmt.Errorf("plugin file %q is too large", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("read plugin file %q: %w", name, err)
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxPluginUnpackedBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read plugin file %q: %w", name, err)
		}
		if int64(len(body)) > maxPluginUnpackedBytes {
			return nil, fmt.Errorf("plugin file %q is too large", name)
		}
		total += int64(len(body))
		if total > maxPluginUnpackedBytes {
			return nil, errors.New("plugin package is too large after decompression")
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("duplicate plugin file %q", name)
		}
		files[name] = body
	}
	return files, nil
}

func ManageInstall(data []byte) (plugin.Info, error) {
	if wasmManager == nil {
		return plugin.Info{}, plugin.ErrDisabled
	}
	return wasmManager.install(data)
}
