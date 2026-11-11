package infra

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

func buildWasmPlugin(t *testing.T, name string) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping wasm plugin build in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}
	out := filepath.Join(t.TempDir(), name+".wasm")
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", out, "./cmd/"+name)
	cmd.Dir = "../../hooks/wasm_plugins"
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", name, err, b)
	}
	return out
}

func writeBuiltPlugin(t *testing.T, wasmPath string, manifest map[string]any) *wasmArtifact {
	t.Helper()
	id, _ := manifest["id"].(string)
	dir := filepath.Join(t.TempDir(), id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.wasm"), wasm, 0o600); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	art, err := loadArtifact(dir)
	if err != nil {
		t.Fatal(err)
	}
	return art
}

func TestDebugWasmPlugin(t *testing.T) {
	wasmPath := buildWasmPlugin(t, "debug")
	art := writeBuiltPlugin(t, wasmPath, map[string]any{
		"id":           "debug",
		"name":         "Debug",
		"version":      "1.0.0",
		"abi":          "pmail-hook@1",
		"hooks":        []string{"SendBefore", "SendAfter", "ReceiveParseBefore", "ReceiveParseAfter", "ReceiveSaveAfter", "GetName", "SettingsHtml"},
		"capabilities": []string{"log", "email.read", "email.write"},
		"limits": map[string]any{
			"memory_mb":        128,
			"timeout_ms":       5000,
			"async_timeout_ms": 10000,
			"output_bytes":     262144,
		},
	})
	grants := map[string]bool{capLog: true, capEmailRead: true, capEmailWrite: true}
	exec, err := newWasmExecutor(context.Background(), art, grants)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "debug",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseAfter": true, "GetName": true, "SettingsHtml": true},
		grantedCaps:   grants,
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}

	email := &parsemail.Email{Subject: "debug-test", Text: []byte("body")}
	hook.ReceiveParseAfter(&pcontext.Context{}, email)
	if email.Status != 5 {
		t.Fatalf("status = %d, want 5", email.Status)
	}
	if email.Subject != "debug-test" || string(email.Text) != "body" {
		t.Fatalf("email fields must be preserved: %+v", email)
	}
	if name := hook.GetName(&pcontext.Context{}); name != "debug" {
		t.Fatalf("name = %q", name)
	}
	if html := hook.SettingsHtml(&pcontext.Context{}, "/x", ""); !strings.Contains(html, "Debug") {
		t.Fatalf("settings html = %q", html)
	}
}

func TestWechatPushWasmPlugin(t *testing.T) {
	wasmPath := buildWasmPlugin(t, "wechat_push")

	var tokenHits, sendHits int
	var sent map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/token":
			tokenHits++
			if r.URL.Query().Get("appid") != "appid" || r.URL.Query().Get("secret") != "secret" {
				t.Errorf("unexpected token query: %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"access_token":"test-token","expires_in":7200}`))
		case "/cgi-bin/message/template/send":
			sendHits++
			if r.URL.Query().Get("access_token") != "test-token" {
				t.Errorf("unexpected access token: %s", r.URL.RawQuery)
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &sent)
			_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)

	art := writeBuiltPlugin(t, wasmPath, map[string]any{
		"id":           "wechat_push",
		"name":         "WeChat Push",
		"version":      "1.0.0",
		"abi":          "pmail-hook@1",
		"hooks":        []string{"ReceiveSaveAfter", "GetName", "SettingsHtml"},
		"capabilities": []string{"log", "email.read", "http"},
		"http_allow":   []string{u.Host},
		"config": map[string]string{
			"app_id":        "appid",
			"secret":        "secret",
			"template_id":   "tpl-1",
			"user_id":       "wx-user",
			"web_url":       "https://mail.example.com",
			"admin_user_id": "1",
			"api_base":      srv.URL,
		},
		"limits": map[string]any{
			"memory_mb":        128,
			"timeout_ms":       5000,
			"async_timeout_ms": 20000,
			"output_bytes":     262144,
		},
	})
	grants := map[string]bool{capLog: true, capEmailRead: true, capHTTP: true}
	exec, err := newWasmExecutor(context.Background(), art, grants)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "wechat_push",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveSaveAfter": true},
		grantedCaps:   grants,
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}

	email := &parsemail.Email{
		Subject:   "登录验证",
		Text:      []byte("您的验证码是 123456，5分钟内有效"),
		MessageId: 1,
	}
	ue := &models.UserEmail{UserID: 1, IsRead: 0, Status: 0}
	hook.ReceiveSaveAfter(&pcontext.Context{}, email, []*models.UserEmail{ue})
	if tokenHits != 1 || sendHits != 1 {
		t.Fatalf("tokenHits=%d sendHits=%d", tokenHits, sendHits)
	}
	if sent["touser"] != "wx-user" || sent["template_id"] != "tpl-1" || sent["url"] != "https://mail.example.com" {
		t.Fatalf("unexpected send payload: %+v", sent)
	}
	data, _ := sent["data"].(map[string]any)
	content, _ := data["Content"].(map[string]any)
	value, _ := content["value"].(string)
	if !strings.Contains(value, "验证码:123456") {
		t.Fatalf("unexpected content: %q", value)
	}
}
