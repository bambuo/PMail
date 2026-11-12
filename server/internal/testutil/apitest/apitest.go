// Package apitest 为 api 子包测试提供与服务启动一致的最小依赖装配。
package apitest

import (
	"context"
	"net/http"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra/mail"
	"github.com/Jinnrry/pmail/internal/infra/mfa"
	"github.com/Jinnrry/pmail/internal/infra/persistence"
	"github.com/Jinnrry/pmail/internal/infra/plugin"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	"xorm.io/xorm"
)

var mfaService *mfa.Service
var repository *persistence.PersistenceRepository
var emailDeps app.EmailServiceDeps

type sessions struct{}

func (sessions) Put(context.Context, string, any) {}
func (sessions) Get(context.Context, string) any  { return nil }
func (sessions) Clear(context.Context) error      { return nil }
func (sessions) LoadAndSave(h http.Handler) http.Handler {
	return h
}

func BindDependencies() {
	repository = persistence.New(nil)
	repo := repository
	app.Sessions = sessions{}
	app.UserSvc = app.NewUserService(repo)
	app.EmailQuerySvc = app.NewEmailQueryService(repo, repo)
	app.RuleSvc = app.NewRuleService(repo, repo)
	app.GroupSvc = app.NewGroupService(repo, repo)
	emailDeps = app.EmailServiceDeps{
		Emails:              repo,
		Users:               repo,
		Audit:               repo,
		OpenDeletionSession: repo.NewDeletionSession,
		NotifyMailbox:       func(*pcontext.Context, int, *models.Email) error { return nil },
		CheckSPF:            mail.CheckSPF,
		Hooks:               plugin.AllHooks,
		DeliveryError:       func(err error, _ map[string]error) error { return err },
		Rules:               app.RuleSvc,
	}
	app.EmailSvc = app.NewEmailService(emailDeps)
	initialize := func(version string) error {
		engine, err := persistence.InitDatabase(version)
		if err != nil {
			return err
		}
		BindDatabase(engine)
		return nil
	}
	app.SetupSvc = app.NewSetupService(repo, initialize, mail.GenerateDKIM)
	app.PluginSvc = app.NewPluginService(app.PluginPorts{
		List:      plugin.ManageList,
		Approve:   plugin.ManageApprove,
		Enable:    plugin.ManageEnable,
		Disable:   plugin.ManageDisable,
		Reject:    plugin.ManageReject,
		Reload:    plugin.ManageReload,
		Install:   plugin.ManageInstall,
		HookNames: plugin.HookNames,
		GetHook:   plugin.GetHook,
	})
	mfaService = mfa.New(nil)
	app.MFASvc = app.NewMFAService(app.MFAPorts{
		Setup:           mfaService.Setup,
		Confirm:         mfaService.Confirm,
		Disable:         mfaService.Disable,
		Reset:           mfaService.Reset,
		Status:          mfaService.Status,
		CreateChallenge: mfaService.CreateChallenge,
		VerifyChallenge: mfaService.VerifyChallenge,
	})
}

func BindDatabase(engine *xorm.Engine) {
	if repository != nil {
		repository.SetEngine(engine)
	}
	if mfaService != nil {
		mfaService.SetEngine(engine)
	}
}

// SetDeliveryError 允许协议测试替换邮件投递错误映射。
func SetDeliveryError(fn func(error, map[string]error) error) {
	emailDeps.DeliveryError = fn
	app.EmailSvc = app.NewEmailService(emailDeps)
}
