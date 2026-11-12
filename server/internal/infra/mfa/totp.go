package mfa

import (
	"crypto/subtle"
	"encoding/json"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/models"
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
