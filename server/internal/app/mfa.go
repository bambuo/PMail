package app

import (
	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

// MFAPorts 两步验证能力，由 bootstrap 绑定到 infra 实现。
type MFAPorts struct {
	Setup           func(*pcontext.Context, string) (*mfadto.SetupData, error)
	Confirm         func(*pcontext.Context, string, string) ([]string, error)
	Disable         func(*pcontext.Context, string, string) error
	Reset           func(*pcontext.Context, string, string) error
	Status          func(*pcontext.Context) (*mfadto.Status, error)
	CreateChallenge func(int) (string, error)
	VerifyChallenge func(string, string) (*models.User, error)
}

// MFAService 两步验证用例：绑定、确认、关闭、重置与登录挑战。
type MFAService struct {
	ports MFAPorts
}

func NewMFAService(ports MFAPorts) *MFAService {
	return &MFAService{ports: ports}
}

// MFASvc 两步验证服务实例，由 bootstrap.BindDependencies 装配
var MFASvc *MFAService

func (s *MFAService) Setup(ctx *pcontext.Context, account string) (*mfadto.SetupData, error) {
	if s.ports.Setup == nil {
		return nil, mfadto.ErrDisabled
	}
	return s.ports.Setup(ctx, account)
}

func (s *MFAService) Confirm(ctx *pcontext.Context, account, code string) ([]string, error) {
	if s.ports.Confirm == nil {
		return nil, mfadto.ErrDisabled
	}
	return s.ports.Confirm(ctx, account, code)
}

func (s *MFAService) Disable(ctx *pcontext.Context, password, code string) error {
	if s.ports.Disable == nil {
		return mfadto.ErrDisabled
	}
	return s.ports.Disable(ctx, password, code)
}

func (s *MFAService) Reset(ctx *pcontext.Context, account, operatorPassword string) error {
	if s.ports.Reset == nil {
		return mfadto.ErrDisabled
	}
	return s.ports.Reset(ctx, account, operatorPassword)
}

func (s *MFAService) Status(ctx *pcontext.Context) (*mfadto.Status, error) {
	if s.ports.Status == nil {
		return nil, mfadto.ErrDisabled
	}
	return s.ports.Status(ctx)
}

func (s *MFAService) CreateChallenge(userID int) (string, error) {
	if s.ports.CreateChallenge == nil {
		return "", mfadto.ErrDisabled
	}
	return s.ports.CreateChallenge(userID)
}

func (s *MFAService) VerifyChallenge(token, code string) (*models.User, error) {
	if s.ports.VerifyChallenge == nil {
		return nil, mfadto.ErrDisabled
	}
	return s.ports.VerifyChallenge(token, code)
}

func MFAIsEnabled(user *models.User) bool {
	return user != nil && user.MfaEnabled == 1
}
