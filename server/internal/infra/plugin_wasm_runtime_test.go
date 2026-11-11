package infra

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/hooks/wasmabi"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

func uleb(n uint32) []byte {
	var out []byte
	for {
		b := byte(n & 0x7f)
		n >>= 7
		if n != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			return out
		}
	}
}

func wasmName(s string) []byte {
	b := []byte(s)
	return append(uleb(uint32(len(b))), b...)
}

func wasmSection(id byte, payload []byte) []byte {
	out := []byte{id}
	out = append(out, uleb(uint32(len(payload)))...)
	return append(out, payload...)
}

func wasmVec(items ...[]byte) []byte {
	out := uleb(uint32(len(items)))
	for _, item := range items {
		out = append(out, item...)
	}
	return out
}

func funcType(params, results []byte) []byte {
	out := []byte{0x60}
	out = append(out, uleb(uint32(len(params)))...)
	out = append(out, params...)
	out = append(out, uleb(uint32(len(results)))...)
	return append(out, results...)
}

type wasmFixture struct {
	response    []byte
	spin        bool
	importLog   bool
	omitExports bool
	memoryPages uint32
}

// buildWasm 手工编码最小测试模块：导入 env.pmail_emit，导出 alloc/free/hook 与 memory。
func buildWasm(t *testing.T, f wasmFixture) []byte {
	t.Helper()
	const (
		i32 = 0x7f
	)
	types := wasmVec(
		funcType([]byte{i32, i32}, []byte{i32}),
		funcType([]byte{i32}, []byte{i32}),
		funcType([]byte{i32, i32}, nil),
		funcType([]byte{i32, i32, i32}, []byte{i32}),
		funcType([]byte{i32, i32, i32}, nil),
	)
	imports := [][]byte{
		append(append(wasmName("env"), wasmName("pmail_emit")...), 0x00, 0x00),
	}
	if f.importLog {
		imports = append(imports, append(append(wasmName("env"), wasmName("pmail_log")...), 0x00, 0x04))
	}
	imported := uint32(len(imports))
	functions := wasmVec([]byte{0x01}, []byte{0x02}, []byte{0x03})
	pages := f.memoryPages
	if pages == 0 {
		pages = 1
	}
	memory := wasmVec(append([]byte{0x00}, uleb(pages)...))

	exports := [][]byte{
		append(wasmName("memory"), 0x02, 0x00),
	}
	if !f.omitExports {
		exports = append(exports,
			append(wasmName("pmail_alloc"), 0x00, byte(imported+0)),
			append(wasmName("pmail_free"), 0x00, byte(imported+1)),
			append(wasmName("pmail_hook"), 0x00, byte(imported+2)),
		)
	}

	allocBody := []byte{0x00, 0x41, 0x80, 0x08, 0x0b}
	freeBody := []byte{0x00, 0x0b}
	var hookBody []byte
	switch {
	case f.spin:
		hookBody = []byte{0x00, 0x03, 0x40, 0x0c, 0x00, 0x0b, 0x41, 0x00, 0x0b}
	case f.response != nil:
		hookBody = []byte{0x00, 0x41, 0x10, 0x41}
		hookBody = append(hookBody, uleb(uint32(len(f.response)))...)
		hookBody = append(hookBody, 0x10, 0x00, 0x0b)
	default:
		hookBody = []byte{0x00, 0x41, 0x00, 0x0b}
	}
	code := wasmVec(
		append(uleb(uint32(len(allocBody))), allocBody...),
		append(uleb(uint32(len(freeBody))), freeBody...),
		append(uleb(uint32(len(hookBody))), hookBody...),
	)

	out := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	out = append(out, wasmSection(1, types)...)
	out = append(out, wasmSection(2, wasmVec(imports...))...)
	out = append(out, wasmSection(3, functions)...)
	out = append(out, wasmSection(5, memory)...)
	out = append(out, wasmSection(7, wasmVec(exports...))...)
	out = append(out, wasmSection(10, code)...)
	if f.response != nil {
		data := wasmVec(append([]byte{0x00, 0x41, 0x10, 0x0b}, append(uleb(uint32(len(f.response))), f.response...)...))
		out = append(out, wasmSection(11, data)...)
	}
	return out
}

func validManifest(id string) *WasmManifest {
	return &WasmManifest{
		ID:           id,
		Name:         "Echo",
		Version:      "1.0.0",
		ABI:          wasmabi.ABI,
		Hooks:        []string{"GetName"},
		Capabilities: []string{"log"},
	}
}

func testArtifact(t *testing.T, id string, wasm []byte, mutate func(*WasmManifest)) *wasmArtifact {
	t.Helper()
	dir := filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	m := validManifest(id)
	if mutate != nil {
		mutate(m)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, m.Entry), wasm, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return &wasmArtifact{Dir: dir, Manifest: m, Wasm: wasm}
}

func TestWasmManifestValidate(t *testing.T) {
	base := validManifest("echo")
	if err := base.Validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if base.Entry != "plugin.wasm" || base.Limits.MemoryMB != 64 {
		t.Fatalf("defaults not applied: %+v", base)
	}
	cases := []struct {
		name string
		edit func(*WasmManifest)
	}{
		{"bad id", func(m *WasmManifest) { m.ID = "Echo" }},
		{"empty name", func(m *WasmManifest) { m.Name = "" }},
		{"bad abi", func(m *WasmManifest) { m.ABI = "pmail-hook@2" }},
		{"empty hooks", func(m *WasmManifest) { m.Hooks = nil }},
		{"unknown hook", func(m *WasmManifest) { m.Hooks = []string{"Unknown"} }},
		{"duplicate hook", func(m *WasmManifest) { m.Hooks = []string{"GetName", "GetName"} }},
		{"unknown cap", func(m *WasmManifest) { m.Capabilities = []string{"http"} }},
		{"mounts", func(m *WasmManifest) { m.Mounts = []WasmMount{{Name: "x", Path: "/x"}} }},
		{"memory", func(m *WasmManifest) { m.Limits.MemoryMB = 999 }},
		{"timeout", func(m *WasmManifest) { m.Limits.TimeoutMS = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest("echo")
			tc.edit(m)
			if err := m.Validate(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestWasmManifestEntryPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.wasm"), []byte{1}, 0o600); err != nil {
		t.Fatal(err)
	}
	m := validManifest("echo")
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if p, err := m.EntryPath(dir); err != nil || p == "" {
		t.Fatalf("entry path: %v", err)
	}
	m.Entry = "../escape.wasm"
	if _, err := m.EntryPath(dir); err == nil {
		t.Fatal("expected escape rejection")
	}
	m.Entry = "/etc/passwd"
	if _, err := m.EntryPath(dir); err == nil {
		t.Fatal("expected absolute rejection")
	}
	if err := os.Symlink(filepath.Join(dir, "plugin.wasm"), filepath.Join(dir, "link.wasm")); err == nil {
		m.Entry = "link.wasm"
		if _, err := m.EntryPath(dir); err == nil {
			t.Fatal("expected symlink rejection")
		}
	}
}

func TestPluginStateStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := NewPluginStateStore(path)
	if err != nil {
		t.Fatal(err)
	}
	st := PluginState{Status: "pending", Version: "1.0.0"}
	if err := s.Put("echo", st); err != nil {
		t.Fatal(err)
	}
	got, ok := s.Get("echo")
	if !ok || got.Status != "pending" {
		t.Fatalf("get: %+v %v", got, ok)
	}
	if err := s.Upsert("echo", func(cur *PluginState) error {
		cur.CallCount = 3
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Get("echo")
	if got.CallCount != 3 || got.Status != "pending" {
		t.Fatalf("upsert overwrote fields: %+v", got)
	}
	if mode := fileMode(t, path); mode != 0o600 {
		t.Fatalf("state file mode = %o", mode)
	}
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPluginStateStore(path); err == nil {
		t.Fatal("expected corrupt state error")
	}
	matches, _ := filepath.Glob(path + ".corrupt-*")
	if len(matches) != 1 {
		t.Fatalf("corrupt backup not created: %v", matches)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func TestWasmExecutorCall(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)})
	art := testArtifact(t, "echo", wasm, nil)
	exec, err := newWasmExecutor(context.Background(), art, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	got, err := exec.Call(context.Background(), wasmabi.HookGetName, []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"name":"echo"}` {
		t.Fatalf("unexpected response: %s", got)
	}
}

func TestWasmExecutorMissingExports(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{omitExports: true})
	art := testArtifact(t, "echo", wasm, nil)
	if _, err := newWasmExecutor(context.Background(), art, nil); err == nil {
		t.Fatal("expected missing exports error")
	}
}

func TestWasmExecutorTimeout(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{spin: true})
	art := testArtifact(t, "spin", wasm, nil)
	exec, err := newWasmExecutor(context.Background(), art, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := exec.Call(ctx, wasmabi.HookGetName, []byte(`{}`))
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected timeout error")
		}
	case <-time.After(5 * time.Second):
		_ = exec.Close(context.Background())
		t.Fatal("timeout did not interrupt execution")
	}
}

func TestWasmExecutorMissingLogImport(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`), importLog: true})
	art := testArtifact(t, "echo", wasm, nil)
	if _, err := newWasmExecutor(context.Background(), art, nil); err == nil {
		t.Fatal("expected missing pmail_log import error")
	}
}

func TestWasmHookGetName(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"my plugin"}`)})
	art := testArtifact(t, "echo", wasm, nil)
	exec, err := newWasmExecutor(context.Background(), art, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"GetName": true},
		grantedCaps:   map[string]bool{},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	if name := hook.GetName(&pcontext.Context{}); name != "myplugin" {
		t.Fatalf("name = %q", name)
	}
}

func TestWasmHookApplyEmail(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"email":{"Subject":"changed","Status":5,"Text":"aGk="}}`)})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveParseAfter"}
		m.Capabilities = []string{"email.read", "email.write"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"email.read": true, "email.write": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseAfter": true},
		grantedCaps:   map[string]bool{"email.read": true, "email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	email := &parsemail.Email{Subject: "orig", Status: 0, Attachments: []*parsemail.Attachment{{Filename: "a.txt"}}}
	hook.ReceiveParseAfter(&pcontext.Context{}, email)
	if email.Subject != "changed" || email.Status != 5 || string(email.Text) != "hi" {
		t.Fatalf("email not applied: %+v", email)
	}
	if len(email.Attachments) != 1 {
		t.Fatal("attachments must be preserved")
	}
}

func TestWasmHookNoWriteGrant(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"email":{"Subject":"changed","Status":5}}`)})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveParseAfter"}
		m.Capabilities = []string{"email.read"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"email.read": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseAfter": true},
		grantedCaps:   map[string]bool{"email.read": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	email := &parsemail.Email{Subject: "orig"}
	hook.ReceiveParseAfter(&pcontext.Context{}, email)
	if email.Subject != "orig" {
		t.Fatalf("email must not change without email.write: %+v", email)
	}
}

func TestRedactEmail(t *testing.T) {
	src := &parsemail.Email{
		Subject:     "s",
		Text:        []byte("text"),
		HTML:        []byte("html"),
		Attachments: []*parsemail.Attachment{{Filename: "a.txt"}},
	}
	out := redactEmail(src)
	if out.Subject != "s" || out.Text != nil || out.HTML != nil || len(out.Attachments) != 0 {
		t.Fatalf("redact failed: %+v", out)
	}
	if src.Text == nil {
		t.Fatal("source must not be mutated")
	}
}

func TestPluginManagerLifecycle(t *testing.T) {
	oldHooks := PluginHookList
	PluginHookList = map[string]contract.EmailHook{}
	defer func() { PluginHookList = oldHooks }()

	root := t.TempDir()
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)})
	dir := filepath.Join(root, "echo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	m := validManifest("echo")
	m.Hooks = []string{"GetName", "ReceiveParseAfter"}
	m.Capabilities = []string{"email.read", "email.write"}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, m.Entry), wasm, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := newPluginManager(root, "test")
	if err := mgr.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer mgr.stop()

	list := mgr.list()
	if len(list) != 1 || list[0].Status != "pending" {
		t.Fatalf("list after init: %+v", list)
	}
	if err := mgr.approve("echo", []string{"ReceiveParseAfter"}, []string{"email.read"}); err != nil {
		t.Fatal(err)
	}
	list = mgr.list()
	if list[0].Status != "approved" || len(list[0].GrantedHooks) != 1 {
		t.Fatalf("list after approve: %+v", list)
	}
	if err := mgr.enable(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := PluginGetHook("echo"); !ok {
		t.Fatal("hook not registered")
	}
	if err := mgr.disable(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := PluginGetHook("echo"); ok {
		t.Fatal("hook not removed")
	}
	if err := mgr.enable(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.reload(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.reject(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	list = mgr.list()
	if list[0].Status != "rejected" || len(list[0].GrantedHooks) != 0 {
		t.Fatalf("list after reject: %+v", list)
	}
	if _, ok := PluginGetHook("echo"); ok {
		t.Fatal("hook not removed after reject")
	}
}

func TestWasmHookReceiveSaveAfter(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveSaveAfter"}
	})
	exec, err := newWasmExecutor(context.Background(), art, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveSaveAfter": true},
		grantedCaps:   map[string]bool{},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	hook.ReceiveSaveAfter(&pcontext.Context{}, &parsemail.Email{}, []*models.UserEmail{{}})
	if hook.counters.callCount.Load() != 1 {
		t.Fatalf("call count = %d", hook.counters.callCount.Load())
	}
}

func TestWasmHookAllHooks(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"SendBefore", "SendAfter", "ReceiveParseBefore", "ReceiveParseAfter", "ReceiveSaveAfter", "GetName", "SettingsHtml"}
		m.Capabilities = []string{"log", "email.read", "email.write"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"log": true, "email.read": true, "email.write": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"SendBefore": true, "SendAfter": true, "ReceiveParseBefore": true, "ReceiveParseAfter": true, "ReceiveSaveAfter": true, "GetName": true, "SettingsHtml": true},
		grantedCaps:   map[string]bool{"log": true, "email.read": true, "email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	email := &parsemail.Email{Subject: "s", Text: []byte("t")}
	raw := []byte("raw")
	hook.SendBefore(&pcontext.Context{}, email)
	hook.SendAfter(&pcontext.Context{}, email, map[string]error{"mx": os.ErrDeadlineExceeded})
	hook.ReceiveParseBefore(&pcontext.Context{}, &raw)
	hook.ReceiveParseAfter(&pcontext.Context{}, email)
	hook.ReceiveSaveAfter(&pcontext.Context{}, email, []*models.UserEmail{{}})
	hook.GetName(&pcontext.Context{})
	hook.SettingsHtml(&pcontext.Context{}, "/api/plugin/settings/echo", "")
	if got := hook.counters.callCount.Load(); got != 7 {
		t.Fatalf("call count = %d, want 7", got)
	}
}

func TestWasmHookReceiveParseBeforeWrite(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"raw":"aGk="}`)})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveParseBefore"}
		m.Capabilities = []string{"email.read", "email.write"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"email.read": true, "email.write": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseBefore": true},
		grantedCaps:   map[string]bool{"email.read": true, "email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	raw := []byte("orig")
	hook.ReceiveParseBefore(&pcontext.Context{}, &raw)
	if string(raw) != "hi" {
		t.Fatalf("raw = %q", raw)
	}
}

func TestPluginManagerBadPluginDoesNotBlock(t *testing.T) {
	oldHooks := PluginHookList
	PluginHookList = map[string]contract.EmailHook{}
	defer func() { PluginHookList = oldHooks }()

	root := t.TempDir()
	bad := filepath.Join(root, "bad")
	if err := os.MkdirAll(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bad, "manifest.json"), []byte(`{"id":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)})
	good := filepath.Join(root, "good")
	if err := os.MkdirAll(good, 0o700); err != nil {
		t.Fatal(err)
	}
	m := validManifest("good")
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(good, m.Entry), wasm, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(good, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	mgr := newPluginManager(root, "test")
	if err := mgr.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer mgr.stop()
	list := mgr.list()
	if len(list) != 1 || list[0].ID != "good" || list[0].Status != "pending" {
		t.Fatalf("unexpected list: %+v", list)
	}
}

func TestWasmHookOutputLimit(t *testing.T) {
	big := make([]byte, 2000)
	for i := range big {
		big[i] = 'a'
	}
	wasm := buildWasm(t, wasmFixture{response: big})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveParseAfter"}
		m.Limits.OutputBytes = 1024
	})
	exec, err := newWasmExecutor(context.Background(), art, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseAfter": true},
		grantedCaps:   map[string]bool{"email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	email := &parsemail.Email{Subject: "orig"}
	hook.ReceiveParseAfter(&pcontext.Context{}, email)
	if email.Subject != "orig" {
		t.Fatalf("email changed: %+v", email)
	}
	if hook.counters.errorCount.Load() != 1 {
		t.Fatalf("error count = %d", hook.counters.errorCount.Load())
	}
}

func TestWasmHookMalformedResponse(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`not-json`)})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveParseAfter"}
		m.Capabilities = []string{"email.write"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"email.write": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseAfter": true},
		grantedCaps:   map[string]bool{"email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	email := &parsemail.Email{Subject: "orig"}
	hook.ReceiveParseAfter(&pcontext.Context{}, email)
	if email.Subject != "orig" {
		t.Fatalf("email changed: %+v", email)
	}
	if hook.counters.callCount.Load() != 1 || hook.counters.errorCount.Load() != 0 {
		t.Fatalf("counters: call=%d err=%d", hook.counters.callCount.Load(), hook.counters.errorCount.Load())
	}
}

func TestPluginManagerApproveInvalidGrant(t *testing.T) {
	root, mgr := newTestManager(t, buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)}), nil)
	_ = root
	defer mgr.stop()
	if err := mgr.approve("echo", []string{"Unknown"}, nil); err == nil {
		t.Fatal("expected invalid grant error")
	}
}

func TestPluginManagerNameConflict(t *testing.T) {
	oldHooks := PluginHookList
	PluginHookList = map[string]contract.EmailHook{"echo": noopHook{}}
	defer func() { PluginHookList = oldHooks }()
	_, mgr := newTestManager(t, buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)}), nil)
	defer mgr.stop()
	if err := mgr.approve("echo", []string{"GetName"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := mgr.enable(context.Background(), "echo"); err == nil {
		t.Fatal("expected name conflict")
	}
}

func TestPluginManagerReloadFailureKeepsOld(t *testing.T) {
	oldHooks := PluginHookList
	PluginHookList = map[string]contract.EmailHook{}
	defer func() { PluginHookList = oldHooks }()
	root, mgr := newTestManager(t, buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)}), nil)
	defer mgr.stop()
	if err := mgr.approve("echo", []string{"GetName"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := mgr.enable(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "echo", "plugin.wasm"), []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.reload(context.Background(), "echo"); err == nil {
		t.Fatal("expected reload failure")
	}
	if _, ok := PluginGetHook("echo"); !ok {
		t.Fatal("old hook must remain registered")
	}
}

func TestWasmHookRawTooLarge(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"ReceiveParseBefore"}
		m.Capabilities = []string{"email.read", "email.write"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"email.read": true, "email.write": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseBefore": true},
		grantedCaps:   map[string]bool{"email.read": true, "email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	raw := make([]byte, maxRawBytes+1)
	hook.ReceiveParseBefore(&pcontext.Context{}, &raw)
	if len(raw) != maxRawBytes+1 {
		t.Fatalf("raw changed: %d", len(raw))
	}
	if hook.counters.errorCount.Load() != 1 {
		t.Fatalf("error count = %d", hook.counters.errorCount.Load())
	}
}

func TestCheckHTTPURL(t *testing.T) {
	allow := map[string]bool{"127.0.0.1:8501": true, "models.example.com": true}
	if _, err := checkHTTPURL("http://127.0.0.1:8501/v1/models/x:predict", allow); err != nil {
		t.Fatalf("allowed URL rejected: %v", err)
	}
	if _, err := checkHTTPURL("https://models.example.com/predict", allow); err != nil {
		t.Fatalf("allowed URL rejected: %v", err)
	}
	for _, raw := range []string{
		"ftp://127.0.0.1:8501/x",
		"http://127.0.0.1:9999/x",
		"http://169.254.169.254/latest/meta-data",
		"http://user:pass@127.0.0.1:8501/x",
	} {
		if _, err := checkHTTPURL(raw, allow); err == nil {
			t.Fatalf("expected rejection: %s", raw)
		}
	}
}

func TestWasmManifestHTTPValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*WasmManifest)
		ok   bool
	}{
		{"http with allow", func(m *WasmManifest) {
			m.Capabilities = []string{"http"}
			m.HTTPAllow = []string{"127.0.0.1:8501"}
		}, true},
		{"http without allow", func(m *WasmManifest) {
			m.Capabilities = []string{"http"}
		}, false},
		{"allow without http", func(m *WasmManifest) {
			m.HTTPAllow = []string{"127.0.0.1:8501"}
		}, false},
		{"bad host", func(m *WasmManifest) {
			m.Capabilities = []string{"http"}
			m.HTTPAllow = []string{"http://127.0.0.1:8501"}
		}, false},
		{"bad config key", func(m *WasmManifest) {
			m.Config = map[string]string{"API-URL": "x"}
		}, false},
		{"long config value", func(m *WasmManifest) {
			m.Config = map[string]string{"api_url": strings.Repeat("x", 1025)}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := validManifest("echo")
			tc.edit(m)
			err := m.Validate()
			if tc.ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestWasmExecutorMemoryLimit(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`), memoryPages: 2000})
	art := testArtifact(t, "echo", wasm, nil)
	if _, err := newWasmExecutor(context.Background(), art, nil); err == nil {
		t.Fatal("expected memory limit error")
	}
}

func TestWasmPayloadRedaction(t *testing.T) {
	hook := &WasmHook{
		id:           "echo",
		manifest:     &WasmManifest{},
		grantedCaps:  map[string]bool{},
		grantedHooks: map[string]bool{},
	}
	email := &parsemail.Email{
		Subject:     "s",
		Text:        []byte("t"),
		HTML:        []byte("h"),
		Attachments: []*parsemail.Attachment{{Filename: "a"}},
	}
	req := hook.request(0, "ReceiveParseAfter", email)
	if req.Email == nil || req.Email.Subject != "s" || req.Email.Text != nil || req.Email.HTML != nil || len(req.Email.Attachments) != 0 {
		t.Fatalf("redaction failed: %+v", req.Email)
	}
	hook.grantedCaps["email.read"] = true
	req = hook.request(0, "ReceiveParseAfter", email)
	if string(req.Email.Text) != "t" || len(req.Email.Attachments) != 1 {
		t.Fatalf("full payload expected: %+v", req.Email)
	}
}

func TestWasmHookUngrantedNotCalled(t *testing.T) {
	hook := &WasmHook{
		id:           "echo",
		manifest:     &WasmManifest{},
		grantedCaps:  map[string]bool{},
		grantedHooks: map[string]bool{},
		counters:     &pluginCounters{},
	}
	hook.SendBefore(&pcontext.Context{}, &parsemail.Email{})
	if html := hook.SettingsHtml(&pcontext.Context{}, "/x", ""); html != "" {
		t.Fatalf("settings html = %q", html)
	}
	if hook.counters.callCount.Load() != 0 {
		t.Fatalf("call count = %d", hook.counters.callCount.Load())
	}
}

func TestPluginManagerEnableFromPending(t *testing.T) {
	_, mgr := newTestManager(t, buildWasm(t, wasmFixture{response: []byte(`{"name":"echo"}`)}), nil)
	defer mgr.stop()
	if err := mgr.enable(context.Background(), "echo"); !errors.Is(err, plugin.ErrInvalidState) {
		t.Fatalf("err = %v", err)
	}
}

func TestPluginManagerManifestChangeResetsPending(t *testing.T) {
	oldHooks := PluginHookList
	PluginHookList = map[string]contract.EmailHook{}
	defer func() { PluginHookList = oldHooks }()

	root, mgr := newTestManager(t, buildWasm(t, wasmFixture{response: []byte(`{"name":"v1"}`)}), nil)
	if err := mgr.approve("echo", []string{"GetName"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := mgr.enable(context.Background(), "echo"); err != nil {
		t.Fatal(err)
	}
	mgr.stop()

	if err := os.WriteFile(filepath.Join(root, "echo", "plugin.wasm"), buildWasm(t, wasmFixture{response: []byte(`{"name":"v2"}`)}), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr2 := newPluginManager(root, "test")
	if err := mgr2.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer mgr2.stop()
	list := mgr2.list()
	if len(list) != 1 || list[0].Status != "pending" {
		t.Fatalf("unexpected list: %+v", list)
	}
	if _, ok := PluginGetHook("echo"); ok {
		t.Fatal("plugin must not load after hash change")
	}
}

func TestPluginWasmInitDisabled(t *testing.T) {
	oldCfg := config.Instance
	oldMgr := wasmManager
	defer func() {
		config.Instance = oldCfg
		wasmManager = oldMgr
	}()
	config.Instance = &config.Config{PluginWasmEnabled: 0}
	wasmManager = nil
	PluginWasmInit("test")
	if wasmManager != nil {
		t.Fatal("manager must stay nil when disabled")
	}
}

func TestWasmHookAsyncReturnIgnored(t *testing.T) {
	wasm := buildWasm(t, wasmFixture{response: []byte(`{"email":{"Subject":"changed"}}`)})
	art := testArtifact(t, "echo", wasm, func(m *WasmManifest) {
		m.Hooks = []string{"SendAfter"}
		m.Capabilities = []string{"email.write"}
	})
	exec, err := newWasmExecutor(context.Background(), art, map[string]bool{"email.write": true})
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "echo",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"SendAfter": true},
		grantedCaps:   map[string]bool{"email.write": true},
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}
	email := &parsemail.Email{Subject: "orig"}
	hook.SendAfter(&pcontext.Context{}, email, nil)
	if email.Subject != "orig" {
		t.Fatalf("async return must be ignored: %+v", email)
	}
	if hook.counters.callCount.Load() != 1 {
		t.Fatalf("call count = %d", hook.counters.callCount.Load())
	}
}

type noopHook struct{}

func (noopHook) SendBefore(*pcontext.Context, *parsemail.Email)                            {}
func (noopHook) SendAfter(*pcontext.Context, *parsemail.Email, map[string]error)           {}
func (noopHook) ReceiveParseBefore(*pcontext.Context, *[]byte)                             {}
func (noopHook) ReceiveParseAfter(*pcontext.Context, *parsemail.Email)                     {}
func (noopHook) ReceiveSaveAfter(*pcontext.Context, *parsemail.Email, []*models.UserEmail) {}
func (noopHook) GetName(*pcontext.Context) string                                          { return "echo" }
func (noopHook) SettingsHtml(*pcontext.Context, string, string) string                     { return "" }

func newTestManager(t *testing.T, wasm []byte, mutate func(*WasmManifest)) (string, *pluginManager) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "echo")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	m := validManifest("echo")
	m.Hooks = []string{"GetName"}
	if mutate != nil {
		mutate(m)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, m.Entry), wasm, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	mgr := newPluginManager(root, "test")
	if err := mgr.init(context.Background()); err != nil {
		t.Fatal(err)
	}
	return root, mgr
}
