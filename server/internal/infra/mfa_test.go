package infra

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/Jinnrry/pmail/config"
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
	if err := InitDatabase("test"); err != nil {
		t.Fatal(err)
	}
	user := &models.User{Account: "mfa-test", Name: "MFA Test", Password: password.Hash("secret")}
	if _, err := Database.Insert(user); err != nil {
		t.Fatal(err)
	}
	ctx := &pcontext.Context{UserID: user.ID, UserAccount: user.Account}
	setup, err := MFASetup(ctx, "")
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
	codes, err := MFAConfirm(ctx, "", code)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if len(codes) != mfaRecoveryCodeCount {
		t.Fatalf("recovery codes = %d, want %d", len(codes), mfaRecoveryCodeCount)
	}
	status, err := MFAStatus(ctx)
	if err != nil || !status.Enabled || status.RecoveryCodesRemaining != mfaRecoveryCodeCount {
		t.Fatalf("unexpected status: %+v err=%v", status, err)
	}
	token, err := MFACreateChallenge(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	loginUser, err := MFAVerifyChallenge(token, code)
	if err != nil || loginUser.ID != user.ID {
		t.Fatalf("verify challenge: user=%+v err=%v", loginUser, err)
	}
	if _, err := MFAVerifyChallenge(token, code); err == nil {
		t.Fatal("expected challenge to be single-use")
	}
	if err := MFADisable(ctx, "secret", codes[0]); err != nil {
		t.Fatalf("disable with recovery code: %v", err)
	}
	status, err = MFAStatus(ctx)
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
	if err := InitDatabase("test"); err != nil {
		t.Fatal(err)
	}
	admin := &models.User{Account: "mfa-admin", Name: "Admin", Password: password.Hash("admin-secret"), IsAdmin: 1}
	regular := &models.User{Account: "mfa-regular", Name: "Regular", Password: password.Hash("user-secret")}
	if _, err := Database.Insert(admin); err != nil {
		t.Fatal(err)
	}
	if _, err := Database.Insert(regular); err != nil {
		t.Fatal(err)
	}
	adminCtx := &pcontext.Context{UserID: admin.ID, UserAccount: admin.Account, IsAdmin: true}

	setup, err := MFASetup(adminCtx, regular.Account)
	if err != nil {
		t.Fatalf("setup for another user: %v", err)
	}
	code, err := totp.GenerateCode(setup.Secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := MFAConfirm(adminCtx, regular.Account, code); err != nil {
		t.Fatalf("confirm for another user: %v", err)
	}

	var regularLoaded models.User
	if _, err := Database.ID(regular.ID).Get(&regularLoaded); err != nil {
		t.Fatal(err)
	}
	if regularLoaded.MfaEnabled != 1 {
		t.Fatal("target user MFA was not enabled")
	}
	var adminLoaded models.User
	if _, err := Database.ID(admin.ID).Get(&adminLoaded); err != nil {
		t.Fatal(err)
	}
	if adminLoaded.MfaEnabled != 0 {
		t.Fatal("operator MFA should not be enabled by a targeted setup")
	}

	if err := MFAReset(adminCtx, regular.Account, "wrong-password"); err == nil {
		t.Fatal("reset with wrong operator password should fail")
	}
	if err := MFAReset(adminCtx, regular.Account, "admin-secret"); err != nil {
		t.Fatalf("reset with operator password: %v", err)
	}
	var regularAfterReset models.User
	if _, err := Database.ID(regular.ID).Get(&regularAfterReset); err != nil {
		t.Fatal(err)
	}
	if regularAfterReset.MfaEnabled != 0 || regularAfterReset.MfaSecret != "" || regularAfterReset.MfaRecoveryCodes != "" {
		t.Fatalf("target user MFA was not cleared: enabled=%d secret=%q codes=%q", regularAfterReset.MfaEnabled, regularAfterReset.MfaSecret, regularAfterReset.MfaRecoveryCodes)
	}
}
