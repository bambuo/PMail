package mfa

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
	"xorm.io/xorm"
)

const (
	mfaIssuer            = "PMail"
	mfaPeriod            = 30
	mfaRecoveryCodeCount = 10
	mfaChallengeTTL      = 5 * time.Minute
	mfaChallengeMaxTries = 5
	mfaKeySize           = 32
)

type Service struct {
	engine *xorm.Engine
}

func New(engine *xorm.Engine) *Service {
	return &Service{engine: engine}
}

func (s *Service) SetEngine(engine *xorm.Engine) {
	s.engine = engine
}

func MfaEnabled(user *models.User) bool {
	return user != nil && user.MfaEnabled == 1
}

func (s *Service) Setup(ctx *pcontext.Context, account string) (*mfadto.SetupData, error) {
	user, err := s.loadUserByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if user.MfaEnabled == 1 {
		return nil, mfadto.ErrAlreadyEnabled
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      mfaIssuer,
		AccountName: user.Account,
		Period:      mfaPeriod,
		SecretSize:  20,
	})
	if err != nil {
		return nil, fmt.Errorf("generate TOTP key: %w", err)
	}
	encrypted, err := encryptMfaSecret(key.Secret())
	if err != nil {
		return nil, err
	}
	if err := s.updateUser(user.ID, map[string]any{
		"mfa_secret":         encrypted,
		"mfa_enabled":        0,
		"mfa_recovery_codes": "",
	}); err != nil {
		return nil, err
	}
	qr, err := qrcode.Encode(key.URL(), qrcode.Medium, 256)
	if err != nil {
		return nil, fmt.Errorf("generate QR code: %w", err)
	}
	return &mfadto.SetupData{
		Secret:    key.Secret(),
		URL:       key.URL(),
		QRDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr),
	}, nil
}

func (s *Service) Confirm(ctx *pcontext.Context, account, code string) ([]string, error) {
	user, err := s.loadUserByAccount(ctx, account)
	if err != nil {
		return nil, err
	}
	if user.MfaEnabled == 1 {
		return nil, mfadto.ErrAlreadyEnabled
	}
	if user.MfaSecret == "" {
		return nil, mfadto.ErrNotEnabled
	}
	ok, err := validateTOTP(user.MfaSecret, code)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, mfadto.ErrInvalidCode
	}
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(hashes)
	if err := s.updateUser(user.ID, map[string]any{
		"mfa_enabled":        1,
		"mfa_recovery_codes": string(raw),
	}); err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *Service) Disable(ctx *pcontext.Context, password, code string) error {
	user, err := s.loadUser(ctx)
	if err != nil {
		return err
	}
	if user.MfaEnabled != 1 {
		return mfadto.ErrNotEnabled
	}
	if !verifyPassword(user.Password, password) {
		return mfadto.ErrPasswordInvalid
	}
	ok, err := s.verifyCode(user, code, true)
	if err != nil {
		return err
	}
	if !ok {
		return mfadto.ErrInvalidCode
	}
	return s.updateUser(user.ID, map[string]any{
		"mfa_secret":         "",
		"mfa_enabled":        0,
		"mfa_recovery_codes": "",
	})
}

func (s *Service) Reset(ctx *pcontext.Context, account, operatorPassword string) error {
	operator, err := s.loadUser(ctx)
	if err != nil {
		return err
	}
	if !verifyPassword(operator.Password, operatorPassword) {
		return mfadto.ErrPasswordInvalid
	}
	target, err := s.loadUserByAccount(ctx, account)
	if err != nil {
		return err
	}
	if target.MfaEnabled != 1 {
		return mfadto.ErrNotEnabled
	}
	return s.updateUser(target.ID, map[string]any{
		"mfa_secret":         "",
		"mfa_enabled":        0,
		"mfa_recovery_codes": "",
	})
}

func (s *Service) Status(ctx *pcontext.Context) (*mfadto.Status, error) {
	user, err := s.loadUser(ctx)
	if err != nil {
		return nil, err
	}
	status := &mfadto.Status{Enabled: user.MfaEnabled == 1}
	if user.MfaRecoveryCodes != "" {
		var hashes []string
		if err := json.Unmarshal([]byte(user.MfaRecoveryCodes), &hashes); err == nil {
			status.RecoveryCodesRemaining = len(hashes)
		}
	}
	return status, nil
}

func (s *Service) loadUser(ctx *pcontext.Context) (*models.User, error) {
	if ctx == nil || ctx.UserID <= 0 {
		return nil, mfadto.ErrNotEnabled
	}
	var user models.User
	ok, err := s.engine.ID(ctx.UserID).Get(&user)
	if err != nil {
		return nil, err
	}
	if !ok || user.ID == 0 {
		return nil, mfadto.ErrNotEnabled
	}
	return &user, nil
}

func (s *Service) loadUserByAccount(ctx *pcontext.Context, account string) (*models.User, error) {
	if account == "" || ctx == nil || account == ctx.UserAccount {
		return s.loadUser(ctx)
	}
	var user models.User
	ok, err := s.engine.Where("account=?", account).Get(&user)
	if err != nil {
		return nil, err
	}
	if !ok || user.ID == 0 {
		return nil, mfadto.ErrUserNotFound
	}
	return &user, nil
}

func (s *Service) updateUser(userID int, fields map[string]any) error {
	_, err := s.engine.Table("user").Where("id=?", userID).Update(fields)
	return err
}
