package mfa

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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/password"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

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

func (s *Service) verifyCode(user *models.User, code string, consumeRecovery bool) (bool, error) {
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
			if err := s.updateUser(user.ID, map[string]any{"mfa_recovery_codes": string(raw)}); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	return false, nil
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
	// 口令存储已支持 bcrypt，兼容历史双重 MD5 哈希
	return password.Verify(encoded, plain)
}
