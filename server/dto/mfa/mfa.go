package mfa

import "errors"

type SetupData struct {
	Secret    string `json:"secret"`
	URL       string `json:"url"`
	QRDataURL string `json:"qrDataUrl"`
}

type Status struct {
	Enabled                bool `json:"enabled"`
	RecoveryCodesRemaining int  `json:"recoveryCodesRemaining"`
}

var (
	ErrDisabled         = errors.New("MFA is not available")
	ErrAlreadyEnabled   = errors.New("MFA is already enabled")
	ErrNotEnabled       = errors.New("MFA is not enabled")
	ErrInvalidCode      = errors.New("invalid MFA code")
	ErrInvalidChallenge = errors.New("MFA challenge is invalid or expired")
	ErrPasswordInvalid  = errors.New("password is incorrect")
	ErrTooManyAttempts  = errors.New("too many MFA attempts")
	ErrUserNotFound     = errors.New("user not found")
)
