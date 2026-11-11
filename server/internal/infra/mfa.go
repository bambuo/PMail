package infra

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Jinnrry/pmail/config"
	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/password"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	mfaIssuer            = "PMail"
	mfaPeriod            = 30
	mfaRecoveryCodeCount = 10
	mfaChallengeTTL      = 5 * time.Minute
	mfaChallengeMaxTries = 5
	mfaKeySize           = 32
)

type mfaChallenge struct {
	userID   int
	expires  time.Time
	attempts int
}

var (
	mfaChallengeMu sync.Mutex
	mfaChallenges  = map[string]*mfaChallenge{}
)

func MfaEnabled(user *models.User) bool {
	return user != nil && user.MfaEnabled == 1
}

func MFASetup(ctx *pcontext.Context, account string) (*mfadto.SetupData, error) {
	user, err := loadMFAUserByAccount(ctx, account)
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
	if err := updateMFAUser(user.ID, map[string]any{
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

func MFAConfirm(ctx *pcontext.Context, account, code string) ([]string, error) {
	user, err := loadMFAUserByAccount(ctx, account)
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
	if err := updateMFAUser(user.ID, map[string]any{
		"mfa_enabled":        1,
		"mfa_recovery_codes": string(raw),
	}); err != nil {
		return nil, err
	}
	return codes, nil
}

func MFADisable(ctx *pcontext.Context, password, code string) error {
	user, err := loadMFAUser(ctx)
	if err != nil {
		return err
	}
	if user.MfaEnabled != 1 {
		return mfadto.ErrNotEnabled
	}
	if !verifyPassword(user.Password, password) {
		return mfadto.ErrPasswordInvalid
	}
	ok, err := verifyUserMFACode(user, code, true)
	if err != nil {
		return err
	}
	if !ok {
		return mfadto.ErrInvalidCode
	}
	return updateMFAUser(user.ID, map[string]any{
		"mfa_secret":         "",
		"mfa_enabled":        0,
		"mfa_recovery_codes": "",
	})
}

func MFAReset(ctx *pcontext.Context, account, operatorPassword string) error {
	operator, err := loadMFAUser(ctx)
	if err != nil {
		return err
	}
	if !verifyPassword(operator.Password, operatorPassword) {
		return mfadto.ErrPasswordInvalid
	}
	target, err := loadMFAUserByAccount(ctx, account)
	if err != nil {
		return err
	}
	if target.MfaEnabled != 1 {
		return mfadto.ErrNotEnabled
	}
	return updateMFAUser(target.ID, map[string]any{
		"mfa_secret":         "",
		"mfa_enabled":        0,
		"mfa_recovery_codes": "",
	})
}

func MFAStatus(ctx *pcontext.Context) (*mfadto.Status, error) {
	user, err := loadMFAUser(ctx)
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

func MFACreateChallenge(userID int) (string, error) {
	var user models.User
	ok, err := Database.ID(userID).Get(&user)
	if err != nil {
		return "", err
	}
	if !ok || user.ID == 0 || user.MfaEnabled != 1 {
		return "", mfadto.ErrNotEnabled
	}
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(tokenBytes)
	now := time.Now()
	mfaChallengeMu.Lock()
	for key, challenge := range mfaChallenges {
		if now.After(challenge.expires) {
			delete(mfaChallenges, key)
		}
	}
	mfaChallenges[token] = &mfaChallenge{userID: userID, expires: now.Add(mfaChallengeTTL)}
	mfaChallengeMu.Unlock()
	return token, nil
}

func MFAVerifyChallenge(token, code string) (*models.User, error) {
	if token == "" || code == "" {
		return nil, mfadto.ErrInvalidChallenge
	}
	mfaChallengeMu.Lock()
	challenge, ok := mfaChallenges[token]
	if !ok {
		mfaChallengeMu.Unlock()
		return nil, mfadto.ErrInvalidChallenge
	}
	if time.Now().After(challenge.expires) {
		delete(mfaChallenges, token)
		mfaChallengeMu.Unlock()
		return nil, mfadto.ErrInvalidChallenge
	}
	if challenge.attempts >= mfaChallengeMaxTries {
		delete(mfaChallenges, token)
		mfaChallengeMu.Unlock()
		return nil, mfadto.ErrTooManyAttempts
	}
	challenge.attempts++
	userID := challenge.userID
	mfaChallengeMu.Unlock()

	var user models.User
	ok, err := Database.ID(userID).Get(&user)
	if err != nil {
		return nil, err
	}
	if !ok || user.ID == 0 || user.Disabled != 0 || user.MfaEnabled != 1 {
		return nil, mfadto.ErrInvalidChallenge
	}
	valid, err := verifyUserMFACode(&user, code, true)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, mfadto.ErrInvalidCode
	}
	mfaChallengeMu.Lock()
	delete(mfaChallenges, token)
	mfaChallengeMu.Unlock()
	return &user, nil
}

func loadMFAUser(ctx *pcontext.Context) (*models.User, error) {
	if ctx == nil || ctx.UserID <= 0 {
		return nil, mfadto.ErrNotEnabled
	}
	var user models.User
	ok, err := Database.ID(ctx.UserID).Get(&user)
	if err != nil {
		return nil, err
	}
	if !ok || user.ID == 0 {
		return nil, mfadto.ErrNotEnabled
	}
	return &user, nil
}

func loadMFAUserByAccount(ctx *pcontext.Context, account string) (*models.User, error) {
	if account == "" || ctx == nil || account == ctx.UserAccount {
		return loadMFAUser(ctx)
	}
	var user models.User
	ok, err := Database.Where("account=?", account).Get(&user)
	if err != nil {
		return nil, err
	}
	if !ok || user.ID == 0 {
		return nil, mfadto.ErrUserNotFound
	}
	return &user, nil
}

func updateMFAUser(userID int, fields map[string]any) error {
	_, err := Database.Table("user").Where("id=?", userID).Update(fields)
	return err
}

func validateTOTP(encryptedSecret, code string) (bool, error) {
	secret, err := decryptMfaSecret(encryptedSecret)
	if err != nil {
		return false, err
	}
	valid, err := totp.ValidateCustom(strings.TrimSpace(code), secret, time.Now().UTC(), totp.ValidateOpts{
		Period:    mfaPeriod,
		Skew:      1,
		Digits:    otp.DigitsSix,
		Algorithm: otp.AlgorithmSHA1,
	})
	if err != nil {
		// Non-6-digit input is not a TOTP; allow recovery-code verification to run.
		return false, nil
	}
	return valid, nil
}

func verifyUserMFACode(user *models.User, code string, consumeRecovery bool) (bool, error) {
	if user.MfaSecret != "" {
		valid, err := validateTOTP(user.MfaSecret, code)
		if err != nil {
			return false, err
		}
		if valid {
			return true, nil
		}
	}
	if user.MfaRecoveryCodes == "" {
		return false, nil
	}
	var hashes []string
	if err := json.Unmarshal([]byte(user.MfaRecoveryCodes), &hashes); err != nil {
		return false, err
	}
	candidate := mfaHashRecoveryCode(code)
	for i, hash := range hashes {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(candidate)) != 1 {
			continue
		}
		if consumeRecovery {
			hashes = append(hashes[:i], hashes[i+1:]...)
			raw, _ := json.Marshal(hashes)
			if err := updateMFAUser(user.ID, map[string]any{"mfa_recovery_codes": string(raw)}); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
}

func generateRecoveryCodes() ([]string, []string, error) {
	codes := make([]string, 0, mfaRecoveryCodeCount)
	hashes := make([]string, 0, mfaRecoveryCodeCount)
	for i := 0; i < mfaRecoveryCodeCount; i++ {
		raw := make([]byte, 10)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)[:16]
		formatted := code[:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
		codes = append(codes, formatted)
		hashes = append(hashes, mfaHashRecoveryCode(formatted))
	}
	return codes, hashes, nil
}

func mfaHashRecoveryCode(code string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}

func verifyPassword(encoded, plain string) bool {
	return subtle.ConstantTimeCompare([]byte(encoded), []byte(password.Encode(plain))) == 1
}

func encryptMfaSecret(secret string) (string, error) {
	key, err := loadMFAKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return base64.StdEncoding.EncodeToString(out), nil
}

func decryptMfaSecret(encoded string) (string, error) {
	key, err := loadMFAKey()
	if err != nil {
		return "", err
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("invalid MFA secret")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func loadMFAKey() ([]byte, error) {
	path := filepath.Join(config.ROOT_PATH, "config", "mfa.key")
	if raw, err := os.ReadFile(path); err == nil {
		if len(raw) != mfaKeySize {
			return nil, errors.New("invalid MFA key length")
		}
		return raw, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	key := make([]byte, mfaKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		return os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Write(key); err != nil {
		return nil, err
	}
	return key, nil
}
