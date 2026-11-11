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
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

// TestSpamBlockWasmPlugin 构建 server/hooks/spam_block_wasm，并用 mock 模型服务跑通
// ReceiveParseAfter 的完整链路。构建较慢，-short 时跳过。
func TestSpamBlockWasmPlugin(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping wasm plugin build in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not found")
	}

	dir := t.TempDir()
	wasmPath := filepath.Join(dir, "plugin.wasm")
	cmd := exec.Command("go", "build", "-buildmode=c-shared", "-o", wasmPath, ".")
	cmd.Dir = "../../hooks/spam_block_wasm"
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build plugin: %v\n%s", err, out)
	}

	var modelReply string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "hello") {
			t.Errorf("model request missing email content: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(modelReply))
	}))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}

	pluginDir := filepath.Join(dir, "spam_block")
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.wasm"), wasm, 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]any{
		"id":           "spam_block",
		"name": "SpamBlock",
		"version":      "1.0.0",
		"abi":          "pmail-hook@1",
		"hooks":        []string{"ReceiveParseAfter"},
		"capabilities": []string{"log", "email.read", "email.write", "http"},
		"http_allow":   []string{u.Host},
		"config": map[string]string{
			"api_url":   srv.URL,
			"threshold": "0.20",
		},
		"limits": map[string]any{
			"memory_mb":        128,
			"timeout_ms":       5000,
			"async_timeout_ms": 10000,
			"output_bytes":     262144,
		},
	}
	raw, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginDir, "manifest.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}

	art, err := loadArtifact(pluginDir)
	if err != nil {
		t.Fatal(err)
	}
	grants := map[string]bool{capLog: true, capEmailRead: true, capEmailWrite: true, capHTTP: true}
	exec, err := newWasmExecutor(context.Background(), art, grants)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())
	hook := &WasmHook{
		id:            "spam_block",
		manifest:      art.Manifest,
		grantedHooks:  map[string]bool{"ReceiveParseAfter": true},
		grantedCaps:   grants,
		exec:          exec,
		serverVersion: "test",
		counters:      &pluginCounters{},
	}

	cases := []struct {
		reply  string
		status int
	}{
		{`{"predictions":[[0.05,0.95,0.00]]}`, 5},
		{`{"predictions":[[0.05,0.00,0.95]]}`, 3},
		{`{"predictions":[[0.95,0.03,0.02]]}`, 0},
	}
	for _, tc := range cases {
		modelReply = tc.reply
		email := &parsemail.Email{Subject: "hello", Text: []byte("buy now")}
		hook.ReceiveParseAfter(&pcontext.Context{}, email)
		if email.Status != tc.status {
			t.Fatalf("reply %s: status = %d, want %d (errs=%d)", tc.reply, email.Status, tc.status, hook.counters.errorCount.Load())
		}
		if email.Subject != "hello" || string(email.Text) != "buy now" {
			t.Fatalf("email fields must be preserved: %+v", email)
		}
	}
}
