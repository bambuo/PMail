package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/Jinnrry/pmail/hooks/wasmabi"
	log "github.com/sirupsen/logrus"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

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
