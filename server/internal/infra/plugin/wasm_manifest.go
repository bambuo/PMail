package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jinnrry/pmail/hooks/wasmabi"
)

const (
	maxRequestBytes = 16 << 20
	maxRawBytes     = 8 << 20
	maxLogBytes     = 4096
	maxWasmBytes    = 32 << 20
	hookCallDrain   = 30 * time.Second

	capLog        = "log"
	capEmailRead  = "email.read"
	capEmailWrite = "email.write"
	capHTTP       = "http"
)

var (
	pluginIDPattern  = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{1,63}$`)
	httpHostPattern  = regexp.MustCompile(`^[A-Za-z0-9.-]+(:\d{1,5})?$`)
	configKeyPattern = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)
	allowedHooks     = map[string]bool{
		"SendBefore":         true,
		"SendAfter":          true,
		"ReceiveParseBefore": true,
		"ReceiveParseAfter":  true,
		"ReceiveSaveAfter":   true,
		"GetName":            true,
		"SettingsHtml":       true,
	}
	allowedCaps = map[string]bool{
		capLog:        true,
		capEmailRead:  true,
		capEmailWrite: true,
		capHTTP:       true,
	}
)

type WasmMount struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	ReadOnly bool   `json:"readonly"`
}

type WasmLimits struct {
	MemoryMB       int `json:"memory_mb"`
	TimeoutMS      int `json:"timeout_ms"`
	AsyncTimeoutMS int `json:"async_timeout_ms"`
	OutputBytes    int `json:"output_bytes"`
}

type WasmManifest struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	ABI          string            `json:"abi"`
	Entry        string            `json:"entry"`
	Hooks        []string          `json:"hooks"`
	Capabilities []string          `json:"capabilities"`
	HTTPAllow    []string          `json:"http_allow"`
	Config       map[string]string `json:"config"`
	Mounts       []WasmMount       `json:"mounts"`
	Limits       WasmLimits        `json:"limits"`
}

type wasmArtifact struct {
	Dir         string
	Manifest    *WasmManifest
	ManifestSHA string
	WasmSHA     string
	Wasm        []byte
}

// LoadWasmManifest 读取并校验插件目录下的 manifest.json。
func LoadWasmManifest(dir string) (*WasmManifest, []byte, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, nil, err
	}
	var m WasmManifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, nil, err
	}
	return &m, raw, nil
}

func (m *WasmManifest) Validate() error {
	if !pluginIDPattern.MatchString(m.ID) {
		return fmt.Errorf("invalid id %q", m.ID)
	}
	if n := utf8.RuneCountInString(m.Name); n == 0 || n > 64 {
		return errors.New("name must be 1-64 characters")
	}
	if n := utf8.RuneCountInString(m.Version); n == 0 || n > 32 {
		return errors.New("version must be 1-32 characters")
	}
	if m.ABI != wasmabi.ABI {
		return fmt.Errorf("unsupported abi %q", m.ABI)
	}
	if len(m.Hooks) == 0 {
		return errors.New("hooks must not be empty")
	}
	seenHook := map[string]bool{}
	for _, h := range m.Hooks {
		if !allowedHooks[h] {
			return fmt.Errorf("unsupported hook %q", h)
		}
		if seenHook[h] {
			return fmt.Errorf("duplicate hook %q", h)
		}
		seenHook[h] = true
	}
	seenCap := map[string]bool{}
	for _, c := range m.Capabilities {
		if !allowedCaps[c] {
			return fmt.Errorf("unsupported capability %q", c)
		}
		if seenCap[c] {
			return fmt.Errorf("duplicate capability %q", c)
		}
		seenCap[c] = true
	}
	if seenCap[capHTTP] {
		if len(m.HTTPAllow) == 0 || len(m.HTTPAllow) > 16 {
			return errors.New("http_allow must contain 1-16 hosts when http is granted")
		}
		for i, host := range m.HTTPAllow {
			if !httpHostPattern.MatchString(host) {
				return fmt.Errorf("invalid http_allow host %q", host)
			}
			m.HTTPAllow[i] = strings.ToLower(host)
		}
	} else if len(m.HTTPAllow) > 0 {
		return errors.New("http_allow requires the http capability")
	}
	if len(m.Config) > 16 {
		return errors.New("config must contain at most 16 entries")
	}
	for k, v := range m.Config {
		if !configKeyPattern.MatchString(k) {
			return fmt.Errorf("invalid config key %q", k)
		}
		if len(v) > 1024 {
			return fmt.Errorf("config value for %q is too long", k)
		}
	}
	if len(m.Mounts) != 0 {
		return errors.New("mounts are not supported")
	}
	if m.Entry == "" {
		m.Entry = "plugin.wasm"
	}
	if m.Limits.MemoryMB == 0 {
		m.Limits.MemoryMB = 64
	}
	if m.Limits.TimeoutMS == 0 {
		m.Limits.TimeoutMS = 2000
	}
	if m.Limits.AsyncTimeoutMS == 0 {
		m.Limits.AsyncTimeoutMS = 5000
	}
	if m.Limits.OutputBytes == 0 {
		m.Limits.OutputBytes = 262144
	}
	if m.Limits.MemoryMB < 1 || m.Limits.MemoryMB > 256 {
		return errors.New("limits.memory_mb must be 1-256")
	}
	if m.Limits.TimeoutMS < 100 || m.Limits.TimeoutMS > 5000 {
		return errors.New("limits.timeout_ms must be 100-5000")
	}
	if m.Limits.AsyncTimeoutMS < 100 || m.Limits.AsyncTimeoutMS > 30000 {
		return errors.New("limits.async_timeout_ms must be 100-30000")
	}
	if m.Limits.OutputBytes < 1024 || m.Limits.OutputBytes > 1<<20 {
		return errors.New("limits.output_bytes must be 1024-1048576")
	}
	return nil
}

func (m *WasmManifest) EntryPath(dir string) (string, error) {
	if filepath.IsAbs(m.Entry) {
		return "", errors.New("entry must be relative")
	}
	clean := filepath.Clean(m.Entry)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("entry escapes plugin directory")
	}
	full := filepath.Join(dir, clean)
	rel, err := filepath.Rel(dir, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("entry escapes plugin directory")
	}
	info, err := os.Lstat(full)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", errors.New("entry must be a regular file")
	}
	if info.Size() > maxWasmBytes {
		return "", errors.New("wasm file too large")
	}
	return full, nil
}

func scanWasmPlugins(root string) ([]*wasmArtifact, []error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []error{err}
	}
	var out []*wasmArtifact
	var errs []error
	seen := map[string]string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil {
			continue
		}
		m, raw, err := LoadWasmManifest(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		if m.ID != entry.Name() {
			errs = append(errs, fmt.Errorf("%s: id %q must equal directory name", entry.Name(), m.ID))
			continue
		}
		if other, ok := seen[m.ID]; ok {
			errs = append(errs, fmt.Errorf("duplicate plugin id %q in %s and %s", m.ID, other, entry.Name()))
			continue
		}
		entryPath, err := m.EntryPath(dir)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		wasm, err := os.ReadFile(entryPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: read entry: %w", entry.Name(), err))
			continue
		}
		seen[m.ID] = entry.Name()
		mSHA := sha256.Sum256(raw)
		wSHA := sha256.Sum256(wasm)
		out = append(out, &wasmArtifact{
			Dir:         dir,
			Manifest:    m,
			ManifestSHA: hex.EncodeToString(mSHA[:]),
			WasmSHA:     hex.EncodeToString(wSHA[:]),
			Wasm:        wasm,
		})
	}
	return out, errs
}
