package app

import (
	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

var (
	SetupMFAFn           func(*pcontext.Context, string) (*mfadto.SetupData, error)
	ConfirmMFAFn         func(*pcontext.Context, string, string) ([]string, error)
	DisableMFAFn         func(*pcontext.Context, string, string) error
	ResetMFAFn           func(*pcontext.Context, string, string) error
	StatusMFAFn          func(*pcontext.Context) (*mfadto.Status, error)
	CreateMFAChallengeFn func(int) (string, error)
	VerifyMFAChallengeFn func(string, string) (*models.User, error)
)

func SetupMFA(ctx *pcontext.Context, account string) (*mfadto.SetupData, error) {
	if SetupMFAFn == nil {
		return nil, mfadto.ErrDisabled
	}
	return SetupMFAFn(ctx, account)
}

func ConfirmMFA(ctx *pcontext.Context, account, code string) ([]string, error) {
	if ConfirmMFAFn == nil {
		return nil, mfadto.ErrDisabled
	}
	return ConfirmMFAFn(ctx, account, code)
}

func DisableMFA(ctx *pcontext.Context, password, code string) error {
	if DisableMFAFn == nil {
		return mfadto.ErrDisabled
	}
	return DisableMFAFn(ctx, password, code)
}

func ResetMFA(ctx *pcontext.Context, account, operatorPassword string) error {
	if ResetMFAFn == nil {
		return mfadto.ErrDisabled
	}
	return ResetMFAFn(ctx, account, operatorPassword)
}

func StatusMFA(ctx *pcontext.Context) (*mfadto.Status, error) {
	if StatusMFAFn == nil {
		return nil, mfadto.ErrDisabled
	}
	return StatusMFAFn(ctx)
}

func CreateMFAChallenge(userID int) (string, error) {
	if CreateMFAChallengeFn == nil {
		return "", mfadto.ErrDisabled
	}
	return CreateMFAChallengeFn(userID)
}

func VerifyMFAChallenge(token, code string) (*models.User, error) {
	if VerifyMFAChallengeFn == nil {
		return nil, mfadto.ErrDisabled
	}
	return VerifyMFAChallengeFn(token, code)
}

func MFAIsEnabled(user *models.User) bool {
	return user != nil && user.MfaEnabled == 1
}
