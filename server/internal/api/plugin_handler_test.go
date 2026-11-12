package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

type pluginListEnvelope struct {
	ErrorNo  int                       `json:"errorNo"`
	ErrorMsg string                    `json:"errorMsg"`
	Data     pluginListResponsePayload `json:"data"`
}

type pluginListResponsePayload struct {
	Enabled bool          `json:"enabled"`
	Plugins []plugin.Info `json:"plugins"`
}

func setupPluginAPITest(t *testing.T) *app.PluginPorts {
	t.Helper()
	oldCfg := config.Instance
	config.Instance = &config.Config{PluginWasmEnabled: 1}
	t.Cleanup(func() { config.Instance = oldCfg })
	return &app.PluginPorts{}
}

func pluginHandlerWith(ports app.PluginPorts) *PluginHandler {
	return &PluginHandler{plugins: app.NewPluginService(ports)}
}

func TestPluginManageRoutesRegistered(t *testing.T) {
	mux := http.NewServeMux()
	httpServerRouter(mux)
	paths := []string{
		"/api/plugin/manage/list",
		"/api/plugin/manage/approve",
		"/api/plugin/manage/enable",
		"/api/plugin/manage/disable",
		"/api/plugin/manage/reject",
		"/api/plugin/manage/reload",
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		_, pattern := mux.Handler(req)
		if pattern != path {
			t.Fatalf("route %s pattern = %q", path, pattern)
		}
	}
}

func TestPluginManageListRequiresAdmin(t *testing.T) {
	setupPluginAPITest(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/plugin/manage/list", nil)
	pluginHandlerWith(app.PluginPorts{}).ManageList(&pcontext.Context{IsAdmin: false}, rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ErrorNo != response.NoAccessPrivileges {
		t.Fatalf("errorNo = %d", resp.ErrorNo)
	}
}

func TestPluginManageListOK(t *testing.T) {
	ports := setupPluginAPITest(t)
	ports.List = func() []plugin.Info {
		return []plugin.Info{{ID: "echo", Name: "Echo", Type: "wasm", Status: plugin.StatusPending}}
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/plugin/manage/list", nil)
	pluginHandlerWith(*ports).ManageList(&pcontext.Context{IsAdmin: true}, rec, req)

	var out pluginListEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.ErrorNo != 0 || !out.Data.Enabled || len(out.Data.Plugins) != 1 {
		t.Fatalf("unexpected response: %+v", out)
	}
}

func TestPluginManageApproveMethodGuard(t *testing.T) {
	setupPluginAPITest(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/plugin/manage/approve", nil)
	pluginHandlerWith(app.PluginPorts{}).ManageApprove(&pcontext.Context{IsAdmin: true}, rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ErrorNo != response.ParamsError {
		t.Fatalf("errorNo = %d", resp.ErrorNo)
	}
}

func TestPluginManageApproveOK(t *testing.T) {
	ports := setupPluginAPITest(t)
	var gotID string
	var gotHooks, gotCaps []string
	ports.Approve = func(ctx *pcontext.Context, id string, hooks, capabilities []string) error {
		gotID, gotHooks, gotCaps = id, hooks, capabilities
		return nil
	}
	body := `{"pluginId":"echo","hooks":["ReceiveParseAfter"],"capabilities":["email.read"]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/plugin/manage/approve", strings.NewReader(body))
	pluginHandlerWith(*ports).ManageApprove(&pcontext.Context{IsAdmin: true}, rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ErrorNo != 0 {
		t.Fatalf("errorNo = %d msg=%s", resp.ErrorNo, resp.ErrorMsg)
	}
	if gotID != "echo" || len(gotHooks) != 1 || len(gotCaps) != 1 {
		t.Fatalf("unexpected args: %s %v %v", gotID, gotHooks, gotCaps)
	}
}

func TestPluginManageApproveBadBody(t *testing.T) {
	setupPluginAPITest(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/plugin/manage/approve", strings.NewReader("{"))
	pluginHandlerWith(app.PluginPorts{}).ManageApprove(&pcontext.Context{IsAdmin: true}, rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ErrorNo != response.ParamsError {
		t.Fatalf("errorNo = %d", resp.ErrorNo)
	}
}

func TestPluginManageEnableOK(t *testing.T) {
	ports := setupPluginAPITest(t)
	called := false
	ports.Enable = func(ctx *pcontext.Context, id string) error {
		called = true
		return nil
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/plugin/manage/enable", strings.NewReader(`{"pluginId":"echo"}`))
	pluginHandlerWith(*ports).ManageEnable(&pcontext.Context{IsAdmin: true}, rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ErrorNo != 0 || !called {
		t.Fatalf("errorNo = %d called=%v", resp.ErrorNo, called)
	}
}

func TestPluginManageEnableInvalidState(t *testing.T) {
	ports := setupPluginAPITest(t)
	ports.Enable = func(ctx *pcontext.Context, id string) error {
		return plugin.ErrInvalidState
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/plugin/manage/enable", strings.NewReader(`{"pluginId":"echo"}`))
	pluginHandlerWith(*ports).ManageEnable(&pcontext.Context{IsAdmin: true}, rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ErrorNo != response.ParamsError {
		t.Fatalf("errorNo = %d", resp.ErrorNo)
	}
}
