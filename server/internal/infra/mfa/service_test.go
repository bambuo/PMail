package mfa

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/infra/persistence"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/password"
	"github.com/pquerna/otp/totp"
)

func TestMFAFlow(t *testing.T) {
	root := t.TempDir()
	oldRoot := config.ROOT_PATH
	oldInstance := config.Instance
	config.ROOT_PATH = root + "/"
	config.Instance = &config.Config{
		DbType:   "sqlite",
		DbDSN:    filepath.Join(root, "mfa.db"),
		LogLevel: "error",
	}
	defer func() {
		config.ROOT_PATH = oldRoot
		config.Instance = oldInstance
	}()
	engine, err := persistence.InitDatabase("test")
	if err != nil {
		t.Fatal(err)
	}
	user := &models.User{Account: "mfa-test", Name: "MFA Test", Password: password.Hash("secret")}
	if _, err := engine.Insert(user); err != nil {
		t.Fatal(err)
	}
	svc := New(engine)
	ctx := &pcontext.Context{UserID: user.ID, UserAccount: user.Account}
	setup, err := svc.Setup(ctx, "")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if setup.Secret == "" || setup.QRDataURL == "" {
		t.Fatalf("unexpected setup data: %+v", setup)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	codes, err := svc.Confirm(ctx, "", code)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if len(codes) != mfaRecoveryCodeCount {
		t.Fatalf("recovery codes = %d, want %d", len(codes), mfaRecoveryCodeCount)
	}
	status, err := svc.Status(ctx)
	if err != nil || !status.Enabled || status.RecoveryCodesRemaining != mfaRecoveryCodeCount {
		t.Fatalf("unexpected status: %+v err=%v", status, err)
	}
	token, err := svc.CreateChallenge(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	loginUser, err := svc.VerifyChallenge(token, code)
	if err != nil || loginUser.ID != user.ID {
		t.Fatalf("verify challenge: user=%+v err=%v", loginUser, err)
	}
	if _, err := svc.VerifyChallenge(token, code); err == nil {
		t.Fatal("expected challenge to be single-use")
	}
	if err := svc.Disable(ctx, "secret", codes[0]); err != nil {
		t.Fatalf("disable with recovery code: %v", err)
	}
	status, err = svc.Status(ctx)
	if err != nil || status.Enabled || status.RecoveryCodesRemaining != 0 {
		t.Fatalf("unexpected status after disable: %+v err=%v", status, err)
	}
}

func TestMFAAdminManagesAnotherUser(t *testing.T) {
	root := t.TempDir()
	oldRoot := config.ROOT_PATH
	oldInstance := config.Instance
	config.ROOT_PATH = root + "/"
	config.Instance = &config.Config{
		DbType:   "sqlite",
		DbDSN:    filepath.Join(root, "mfa-admin.db"),
		LogLevel: "error",
	}
	defer func() {
		config.ROOT_PATH = oldRoot
		config.Instance = oldInstance
	}()
	engine, err := persistence.InitDatabase("test")
	if err != nil {
		t.Fatal(err)
	}
	svc := New(engine)
	admin := &models.User{Account: "mfa-admin", Name: "Admin", Password: password.Hash("admin-secret"), IsAdmin: 1}
	regular := &models.User{Account: "mfa-regular", Name: "Regular", Password: password.Hash("user-secret")}
	if _, err := engine.Insert(admin); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Insert(regular); err != nil {
		t.Fatal(err)
	}
	adminCtx := &pcontext.Context{UserID: admin.ID, UserAccount: admin.Account, IsAdmin: true}

	setup, err := svc.Setup(adminCtx, regular.Account)
	if err != nil {
		t.Fatalf("setup for another user: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Confirm(adminCtx, regular.Account, code); err != nil {
		t.Fatalf("confirm for another user: %v", err)
	}

	var regularLoaded models.User
	if _, err := engine.ID(regular.ID).Get(&regularLoaded); err != nil {
		t.Fatal(err)
	}
	if regularLoaded.MfaEnabled != 1 {
		t.Fatal("target user MFA was not enabled")
	}
	var adminLoaded models.User
	if _, err := engine.ID(admin.ID).Get(&adminLoaded); err != nil {
		t.Fatal(err)
	}
	if adminLoaded.MfaEnabled != 0 {
		t.Fatal("operator MFA should not be enabled by a targeted setup")
	}

	if err := svc.Reset(adminCtx, regular.Account, "wrong-password"); err == nil {
		t.Fatal("reset with wrong operator password should fail")
	}
	if err := svc.Reset(adminCtx, regular.Account, "admin-secret"); err != nil {
		t.Fatalf("reset with operator password: %v", err)
	}
	var regularAfterReset models.User
	if _, err := engine.ID(regular.ID).Get(&regularAfterReset); err != nil {
		t.Fatal(err)
	}
	if regularAfterReset.MfaEnabled != 0 || regularAfterReset.MfaSecret != "" || regularAfterReset.MfaRecoveryCodes != "" {
		t.Fatalf("target user MFA was not cleared: enabled=%d secret=%q codes=%q", regularAfterReset.MfaEnabled, regularAfterReset.MfaSecret, regularAfterReset.MfaRecoveryCodes)
	}
}
