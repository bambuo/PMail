package infra

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/hooks/wasmabi"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
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

type wasmExecutor struct {
	mu         sync.Mutex
	art        *wasmArtifact
	granted    map[string]bool
	maxOut     int
	memoryMB   int
	rt         wazero.Runtime
	mod        api.Module
	alloc      api.Function
	free       api.Function
	hook       api.Function
	result     []byte
	broken     bool
	pluginID   string
	httpAllow  map[string]bool
	httpClient *http.Client
}

func newWasmExecutor(ctx context.Context, art *wasmArtifact, granted map[string]bool) (*wasmExecutor, error) {
	allow := map[string]bool{}
	for _, host := range art.Manifest.HTTPAllow {
		allow[strings.ToLower(host)] = true
	}
	e := &wasmExecutor{
		art:       art,
		granted:   granted,
		maxOut:    art.Manifest.Limits.OutputBytes,
		memoryMB:  art.Manifest.Limits.MemoryMB,
		pluginID:  art.Manifest.ID,
		httpAllow: allow,
		httpClient: &http.Client{
			Timeout: time.Duration(art.Manifest.Limits.AsyncTimeoutMS) * time.Millisecond,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	if err := e.init(ctx); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *wasmExecutor) init(ctx context.Context) error {
	e.closeLocked(ctx)
	pages := uint32(e.memoryMB * 16)
	rCfg := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(pages).
		WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(ctx, rCfg)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		rt.CloseWithExitCode(ctx, 0)
		return err
	}
	host := rt.NewHostModuleBuilder(wasmabi.HostModule)
	host.NewFunctionBuilder().WithFunc(func(ptr, size uint32) uint32 {
		return e.emit(ptr, size)
	}).Export(wasmabi.ImportEmit)
	if e.granted[capLog] {
		host.NewFunctionBuilder().WithFunc(func(level, ptr, size uint32) {
			e.logMessage(level, ptr, size)
		}).Export(wasmabi.ImportLog)
	}
	if e.granted[capHTTP] {
		host.NewFunctionBuilder().WithFunc(func(urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap uint32) int32 {
			return e.httpPost(urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap)
		}).Export(wasmabi.ImportHTTP)
		host.NewFunctionBuilder().WithFunc(func(urlPtr, urlLen, outPtr, outCap uint32) int32 {
			return e.httpGet(urlPtr, urlLen, outPtr, outCap)
		}).Export(wasmabi.ImportHTTPGet)
	}
	if _, err := host.Instantiate(ctx); err != nil {
		rt.CloseWithExitCode(ctx, 0)
		return err
	}
	compiled, err := rt.CompileModule(ctx, e.art.Wasm)
	if err != nil {
		rt.CloseWithExitCode(ctx, 0)
		return err
	}
	mod, err := rt.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().
		WithName(e.pluginID).
		WithStartFunctions().
		WithSysWalltime().
		WithSysNanotime().
		WithSysNanosleep())
	if err != nil {
		rt.CloseWithExitCode(ctx, 0)
		return err
	}
	alloc := mod.ExportedFunction(wasmabi.ExportAlloc)
	free := mod.ExportedFunction(wasmabi.ExportFree)
	hook := mod.ExportedFunction(wasmabi.ExportHook)
	if alloc == nil || free == nil || hook == nil {
		mod.CloseWithExitCode(ctx, 0)
		rt.CloseWithExitCode(ctx, 0)
		return errors.New("missing required exports")
	}
	e.rt = rt
	e.mod = mod
	e.alloc = alloc
	e.free = free
	e.hook = hook
	e.broken = false
	if initFn := mod.ExportedFunction("_initialize"); initFn != nil {
		initCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := initFn.Call(initCtx)
		cancel()
		if err != nil {
			e.closeLocked(context.Background())
			return fmt.Errorf("initialize: %w", err)
		}
	}
	return nil
}

func (e *wasmExecutor) closeLocked(ctx context.Context) {
	if e.rt != nil {
		_ = e.rt.CloseWithExitCode(ctx, 0)
	}
	e.rt = nil
	e.mod = nil
	e.alloc = nil
	e.free = nil
	e.hook = nil
}

func (e *wasmExecutor) Close(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.closeLocked(ctx)
	return nil
}

func (e *wasmExecutor) Call(ctx context.Context, hookID uint32, req []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(req) > maxRequestBytes {
		return nil, errors.New("request exceeds 16 MiB")
	}
	if e.broken || e.mod == nil {
		if err := e.init(context.Background()); err != nil {
			return nil, err
		}
	}
	e.result = nil
	res, err := e.alloc.Call(ctx, uint64(len(req)))
	if err != nil {
		e.failLocked()
		return nil, err
	}
	if len(res) == 0 {
		return nil, errors.New("alloc returned no value")
	}
	ptr := uint32(res[0])
	if ptr == 0 {
		return nil, errors.New("alloc failed")
	}
	if !e.mod.Memory().Write(ptr, req) {
		return nil, errors.New("write guest memory failed")
	}
	hookRes, err := e.hook.Call(ctx, uint64(hookID), uint64(ptr), uint64(len(req)))
	_, _ = e.free.Call(context.Background(), uint64(ptr), uint64(len(req)))
	if err != nil {
		e.failLocked()
		return nil, err
	}
	if len(hookRes) == 0 || hookRes[0] != 0 {
		return nil, fmt.Errorf("hook returned error code %v", hookRes)
	}
	return append([]byte(nil), e.result...), nil
}

func (e *wasmExecutor) failLocked() {
	e.broken = true
	e.closeLocked(context.Background())
}

func (e *wasmExecutor) emit(ptr, size uint32) uint32 {
	if size > uint32(e.maxOut) {
		return 1
	}
	b, ok := e.readGuest(ptr, size)
	if !ok {
		return 1
	}
	e.result = append([]byte(nil), b...)
	return 0
}

func (e *wasmExecutor) logMessage(level, ptr, size uint32) {
	b, ok := e.readGuest(ptr, size)
	if !ok {
		return
	}
	if len(b) > maxLogBytes {
		b = b[:maxLogBytes]
	}
	if !utf8.Valid(b) {
		return
	}
	entry := log.WithField("plugin_id", e.pluginID)
	switch level {
	case 0:
		entry.Debug(string(b))
	case 1:
		entry.Info(string(b))
	case 2:
		entry.Warn(string(b))
	default:
		entry.Error(string(b))
	}
}

func (e *wasmExecutor) httpPost(urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap uint32) int32 {
	return e.httpDo(http.MethodPost, urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap)
}

func (e *wasmExecutor) httpGet(urlPtr, urlLen, outPtr, outCap uint32) int32 {
	return e.httpDo(http.MethodGet, urlPtr, urlLen, 0, 0, outPtr, outCap)
}

func (e *wasmExecutor) httpDo(method string, urlPtr, urlLen, bodyPtr, bodyLen, outPtr, outCap uint32) int32 {
	if urlLen == 0 || urlLen > 2048 || bodyLen > 1<<20 || outCap == 0 || int(outCap) > e.maxOut {
		return -1
	}
	rawURL, ok := e.readGuest(urlPtr, urlLen)
	if !ok {
		return -1
	}
	var body []byte
	if bodyLen > 0 {
		body, ok = e.readGuest(bodyPtr, bodyLen)
		if !ok {
			return -1
		}
	}
	target, err := checkHTTPURL(string(rawURL), e.httpAllow)
	if err != nil {
		log.WithField("plugin_id", e.pluginID).Warnf("wasm plugin http denied: %v", err)
		return -1
	}
	var bodyReader io.Reader
	if bodyLen > 0 {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, target.String(), bodyReader)
	if err != nil {
		return -1
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.httpClient.Do(req)
	if err != nil {
		log.WithField("plugin_id", e.pluginID).Warnf("wasm plugin http %s failed: %v", target.Host, err)
		return -2
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return -4
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(outCap)+1))
	if err != nil {
		return -2
	}
	if len(data) > int(outCap) {
		return -3
	}
	if !e.mod.Memory().Write(outPtr, data) {
		return -1
	}
	return int32(len(data))
}

func checkHTTPURL(raw string, allow map[string]bool) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("scheme %q is not allowed", u.Scheme)
	}
	if u.User != nil {
		return nil, errors.New("userinfo is not allowed")
	}
	host := strings.ToLower(u.Host)
	if host == "" {
		return nil, errors.New("host is required")
	}
	if !allow[host] {
		return nil, fmt.Errorf("host %q is not allowed", host)
	}
	return u, nil
}

func (e *wasmExecutor) readGuest(ptr, size uint32) ([]byte, bool) {
	if e.mod == nil {
		return nil, false
	}
	if size == 0 {
		return []byte{}, true
	}
	return e.mod.Memory().Read(ptr, size)
}

type pluginCounters struct {
	callCount  atomic.Int64
	errorCount atomic.Int64
	lastUsedAt atomic.Int64
	lastError  atomic.Value
}

func (c *pluginCounters) record(err error) {
	c.callCount.Add(1)
	c.lastUsedAt.Store(time.Now().Unix())
	if err != nil {
		c.errorCount.Add(1)
		c.lastError.Store(err.Error())
	}
}

func (c *pluginCounters) snapshot() (int64, int64, string, string) {
	last := ""
	if v := c.lastError.Load(); v != nil {
		last, _ = v.(string)
	}
	lastUsed := ""
	if ts := c.lastUsedAt.Load(); ts > 0 {
		lastUsed = time.Unix(ts, 0).Format(time.RFC3339)
	}
	return c.callCount.Load(), c.errorCount.Load(), last, lastUsed
}

// WasmHook 把 WASM 插件适配为 contract.EmailHook。
type WasmHook struct {
	id            string
	manifest      *WasmManifest
	grantedHooks  map[string]bool
	grantedCaps   map[string]bool
	exec          *wasmExecutor
	serverVersion string
	counters      *pluginCounters
	draining      atomic.Bool
	inflight      sync.WaitGroup
}

var _ contract.EmailHook = (*WasmHook)(nil)

func (h *WasmHook) Name() string { return h.id }

func (h *WasmHook) GetName(ctx *pcontext.Context) string {
	resp, err := h.invoke(ctx, wasmabi.HookGetName, &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "GetName",
		ServerVersion: h.serverVersion,
	}, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err != nil {
		return ""
	}
	var r nameResponse
	if json.Unmarshal(resp, &r) != nil {
		return ""
	}
	name := strings.ReplaceAll(r.Name, " ", "")
	if name == "" {
		return h.manifest.Name
	}
	return name
}

func (h *WasmHook) SettingsHtml(ctx *pcontext.Context, url, requestData string) string {
	if !h.grantedHooks["SettingsHtml"] {
		return ""
	}
	resp, err := h.invoke(ctx, wasmabi.HookSettingsHTML, &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "SettingsHtml",
		ServerVersion: h.serverVersion,
		URL:           url,
		RequestData:   requestData,
	}, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err != nil {
		return ""
	}
	var r htmlResponse
	if json.Unmarshal(resp, &r) != nil {
		return ""
	}
	return r.HTML
}

func (h *WasmHook) SendBefore(ctx *pcontext.Context, email *parsemail.Email) {
	if !h.grantedHooks["SendBefore"] {
		return
	}
	req := h.request(wasmabi.HookSendBefore, "SendBefore", email)
	resp, err := h.invoke(ctx, wasmabi.HookSendBefore, req, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err == nil && h.grantedCaps[capEmailWrite] {
		h.applyEmailResponse(email, resp)
	}
}

func (h *WasmHook) SendAfter(ctx *pcontext.Context, email *parsemail.Email, errs map[string]error) {
	if !h.grantedHooks["SendAfter"] {
		return
	}
	req := h.request(wasmabi.HookSendAfter, "SendAfter", email)
	req.Errors = map[string]string{}
	for host, err := range errs {
		if err != nil {
			req.Errors[host] = err.Error()
		}
	}
	_, err := h.invoke(ctx, wasmabi.HookSendAfter, req, time.Duration(h.manifest.Limits.AsyncTimeoutMS)*time.Millisecond)
	h.report(ctx, err)
}

func (h *WasmHook) ReceiveParseBefore(ctx *pcontext.Context, raw *[]byte) {
	if !h.grantedHooks["ReceiveParseBefore"] {
		return
	}
	if raw == nil {
		return
	}
	if len(*raw) > maxRawBytes {
		h.report(ctx, fmt.Errorf("raw exceeds %d bytes", maxRawBytes))
		return
	}
	req := &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          "ReceiveParseBefore",
		ServerVersion: h.serverVersion,
	}
	if h.grantedCaps[capEmailRead] {
		req.Raw = base64.StdEncoding.EncodeToString(*raw)
	}
	resp, err := h.invoke(ctx, wasmabi.HookReceiveParseBefore, req, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err != nil || !h.grantedCaps[capEmailWrite] {
		return
	}
	var r rawResponse
	if json.Unmarshal(resp, &r) != nil || r.Raw == "" {
		return
	}
	decoded, derr := base64.StdEncoding.DecodeString(r.Raw)
	if derr != nil || len(decoded) > maxRawBytes {
		return
	}
	*raw = decoded
}

func (h *WasmHook) ReceiveParseAfter(ctx *pcontext.Context, email *parsemail.Email) {
	if !h.grantedHooks["ReceiveParseAfter"] {
		return
	}
	req := h.request(wasmabi.HookReceiveParseAfter, "ReceiveParseAfter", email)
	resp, err := h.invoke(ctx, wasmabi.HookReceiveParseAfter, req, time.Duration(h.manifest.Limits.TimeoutMS)*time.Millisecond)
	h.report(ctx, err)
	if err == nil && h.grantedCaps[capEmailWrite] {
		h.applyEmailResponse(email, resp)
	}
}

func (h *WasmHook) ReceiveSaveAfter(ctx *pcontext.Context, email *parsemail.Email, ue []*models.UserEmail) {
	if !h.grantedHooks["ReceiveSaveAfter"] {
		return
	}
	req := h.request(wasmabi.HookReceiveSaveAfter, "ReceiveSaveAfter", email)
	req.UserEmail = ue
	_, err := h.invoke(ctx, wasmabi.HookReceiveSaveAfter, req, time.Duration(h.manifest.Limits.AsyncTimeoutMS)*time.Millisecond)
	h.report(ctx, err)
}

func (h *WasmHook) request(hookID uint32, hookName string, email *parsemail.Email) *hookRequest {
	req := &hookRequest{
		ABI:           wasmabi.ABI,
		Hook:          hookName,
		ServerVersion: h.serverVersion,
	}
	if email == nil {
		return req
	}
	if h.grantedCaps[capEmailRead] {
		req.Email = email
	} else {
		req.Email = redactEmail(email)
	}
	return req
}

func (h *WasmHook) invoke(ctx *pcontext.Context, hookID uint32, req *hookRequest, timeout time.Duration) ([]byte, error) {
	if h.draining.Load() {
		return nil, errors.New("plugin is draining")
	}
	h.inflight.Add(1)
	defer h.inflight.Done()
	if req.Config == nil && len(h.manifest.Config) > 0 {
		req.Config = h.manifest.Config
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	base := context.Background()
	if ctx != nil && ctx.Context != nil {
		base = ctx.Context
	}
	callCtx, cancel := context.WithTimeout(base, timeout)
	defer cancel()
	return h.exec.Call(callCtx, hookID, body)
}

func (h *WasmHook) applyEmailResponse(email *parsemail.Email, resp []byte) {
	if email == nil {
		return
	}
	var r emailResponse
	if json.Unmarshal(resp, &r) != nil || r.Email == nil {
		return
	}
	applyEmail(email, r.Email)
}

func (h *WasmHook) report(ctx *pcontext.Context, err error) {
	if h.counters != nil {
		h.counters.record(err)
	}
	if err == nil {
		return
	}
	entry := log.WithFields(log.Fields{
		"plugin_id": h.id,
		"plugin":    h.manifest.Name,
	})
	if ctx != nil {
		entry = entry.WithContext(ctx)
	}
	entry.Errorf("wasm plugin call failed: %v", err)
}

func (h *WasmHook) drain(timeout time.Duration) {
	h.draining.Store(true)
	done := make(chan struct{})
	go func() {
		h.inflight.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

type hookRequest struct {
	ABI           string              `json:"abi"`
	Hook          string              `json:"hook"`
	ServerVersion string              `json:"serverVersion"`
	Email         *parsemail.Email    `json:"email,omitempty"`
	Errors        map[string]string   `json:"errors,omitempty"`
	Raw           string              `json:"raw,omitempty"`
	UserEmail     []*models.UserEmail `json:"userEmail,omitempty"`
	URL           string              `json:"url,omitempty"`
	RequestData   string              `json:"requestData,omitempty"`
	Config        map[string]string   `json:"config,omitempty"`
}

type emailPatch struct {
	Subject     *string               `json:"Subject"`
	Text        *[]byte               `json:"Text"`
	HTML        *[]byte               `json:"HTML"`
	Status      *int                  `json:"Status"`
	From        *parsemail.User       `json:"From"`
	To          *[]*parsemail.User    `json:"To"`
	Cc          *[]*parsemail.User    `json:"Cc"`
	Bcc         *[]*parsemail.User    `json:"Bcc"`
	ReplyTo     *[]*parsemail.User    `json:"ReplyTo"`
	Sender      *parsemail.User       `json:"Sender"`
	Headers     *textproto.MIMEHeader `json:"Headers"`
	Date        *string               `json:"Date"`
	ReadReceipt *[]string             `json:"ReadReceipt"`
}

type emailResponse struct {
	Email *emailPatch `json:"email"`
}

type rawResponse struct {
	Raw string `json:"raw"`
}

type nameResponse struct {
	Name string `json:"name"`
}

type htmlResponse struct {
	HTML string `json:"html"`
}

func redactEmail(src *parsemail.Email) *parsemail.Email {
	out := *src
	out.Text = nil
	out.HTML = nil
	out.Headers = nil
	out.Attachments = nil
	return &out
}

func applyEmail(dst *parsemail.Email, src *emailPatch) {
	if dst == nil || src == nil {
		return
	}
	if src.Subject != nil {
		dst.Subject = *src.Subject
	}
	if src.Text != nil {
		dst.Text = *src.Text
	}
	if src.HTML != nil {
		dst.HTML = *src.HTML
	}
	if src.Status != nil {
		dst.Status = *src.Status
	}
	if src.From != nil {
		dst.From = src.From
	}
	if src.To != nil {
		dst.To = *src.To
	}
	if src.Cc != nil {
		dst.Cc = *src.Cc
	}
	if src.Bcc != nil {
		dst.Bcc = *src.Bcc
	}
	if src.ReplyTo != nil {
		dst.ReplyTo = *src.ReplyTo
	}
	if src.Sender != nil {
		dst.Sender = src.Sender
	}
	if src.Headers != nil {
		dst.Headers = *src.Headers
	}
	if src.Date != nil {
		dst.Date = *src.Date
	}
	if src.ReadReceipt != nil {
		dst.ReadReceipt = *src.ReadReceipt
	}
}
