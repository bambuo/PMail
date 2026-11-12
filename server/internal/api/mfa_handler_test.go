package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

func TestMFASetupTargetsOtherAccountOnlyForAdmin(t *testing.T) {
	called := false
	h := &MFAHandler{mfa: app.NewMFAService(app.MFAPorts{
		Setup: func(ctx *pcontext.Context, account string) (*mfadto.SetupData, error) {
			called = true
			return &mfadto.SetupData{Secret: "test-secret", URL: "url", QRDataURL: "qr"}, nil
		},
	})}

	req := httptest.NewRequest(http.MethodPost, "/api/mfa/setup", strings.NewReader(`{"account":"other"}`))
	rec := httptest.NewRecorder()
	h.Setup(&pcontext.Context{UserID: 1, UserAccount: "basic"}, rec, req)

	var unauthorized response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &unauthorized); err != nil {
		t.Fatal(err)
	}
	if unauthorized.ErrorNo != response.NoAccessPrivileges {
		t.Fatalf("non-admin errorNo = %d, want %d", unauthorized.ErrorNo, response.NoAccessPrivileges)
	}
	if called {
		t.Fatal("setup must not run for another account without admin")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/mfa/setup", strings.NewReader(`{"account":"other"}`))
	rec = httptest.NewRecorder()
	h.Setup(&pcontext.Context{UserID: 2, UserAccount: "admin", IsAdmin: true}, rec, req)

	var ok response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil {
		t.Fatal(err)
	}
	if ok.ErrorNo != 0 {
		t.Fatalf("admin errorNo = %d, msg = %s", ok.ErrorNo, ok.ErrorMsg)
	}
	if !called {
		t.Fatal("admin setup did not reach the app layer")
	}
}

func TestMFAResetRequiresAdmin(t *testing.T) {
	called := false
	h := &MFAHandler{mfa: app.NewMFAService(app.MFAPorts{
		Reset: func(ctx *pcontext.Context, account, password string) error {
			called = true
			return nil
		},
	})}

	req := httptest.NewRequest(http.MethodPost, "/api/mfa/reset", strings.NewReader(`{"account":"other","password":"pwd"}`))
	rec := httptest.NewRecorder()
	h.Reset(&pcontext.Context{UserID: 1, UserAccount: "basic"}, rec, req)

	var unauthorized response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &unauthorized); err != nil {
		t.Fatal(err)
	}
	if unauthorized.ErrorNo != response.NoAccessPrivileges {
		t.Fatalf("non-admin errorNo = %d, want %d", unauthorized.ErrorNo, response.NoAccessPrivileges)
	}
	if called {
		t.Fatal("reset must not run for non-admin")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/mfa/reset", strings.NewReader(`{"account":"other","password":"pwd"}`))
	rec = httptest.NewRecorder()
	h.Reset(&pcontext.Context{UserID: 2, UserAccount: "admin", IsAdmin: true}, rec, req)

	var ok response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &ok); err != nil {
		t.Fatal(err)
	}
	if ok.ErrorNo != 0 || !called {
		t.Fatalf("admin reset errorNo = %d, called = %v", ok.ErrorNo, called)
	}
}
