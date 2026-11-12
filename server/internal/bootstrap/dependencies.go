package bootstrap

import (
	"xorm.io/xorm"

	imapapi "github.com/Jinnrry/pmail/internal/api/imap"
	smtpapi "github.com/Jinnrry/pmail/internal/api/smtp"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra/certificate"
	"github.com/Jinnrry/pmail/internal/infra/mail"
	"github.com/Jinnrry/pmail/internal/infra/mfa"
	"github.com/Jinnrry/pmail/internal/infra/persistence"
	"github.com/Jinnrry/pmail/internal/infra/plugin"
	"github.com/Jinnrry/pmail/pkg/context"
)

var persistenceRepo *persistence.PersistenceRepository
var mfaService *mfa.Service

func BindDependencies() {
	persistenceRepo = persistence.New(nil)
	repo := persistenceRepo

	app.UserSvc = app.NewUserService(repo)
	app.EmailQuerySvc = app.NewEmailQueryService(repo, repo)
	app.RuleSvc = app.NewRuleService(repo, repo)
	app.GroupSvc = app.NewGroupService(repo, repo)
	app.EmailSvc = app.NewEmailService(app.EmailServiceDeps{
		Emails:              repo,
		Users:               repo,
		Audit:               repo,
		OpenDeletionSession: repo.NewDeletionSession,
		NotifyMailbox:       imapapi.NewIMAPIdleNotifier(app.GroupSvc).Notify,
		CheckSPF:            mail.CheckSPF,
		Hooks:               plugin.AllHooks,
		DeliveryError:       smtpapi.DeliveryResultError,
		Rules:               app.RuleSvc,
	})
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

	app.GetCertificateType = certificate.GetSSL
	app.SetCertificate = certificate.SetSSL
	app.GenerateCertificate = certificate.GenSSL
	app.HTTPChallenge = func(token string) (string, bool) {
		info, ok := certificate.GetHttpChallengeInstance().AuthInfo[token]
		if !ok {
			return "", false
		}
		return info.KeyAuth, true
	}
	app.GetDNSChallengeRecords = func(ctx *context.Context) []*app.DNSChallengeRecord {
		records := certificate.GetDnsChallengeInstance().GetDNSSettings(ctx)
		out := make([]*app.DNSChallengeRecord, 0, len(records))
		for _, r := range records {
			out = append(out, &app.DNSChallengeRecord{Type: r.Type, Host: r.Host, Value: r.Value, TTL: r.TTL, Tips: r.Tips})
		}
		return out
	}
	certificate.SetupFinished = app.SetupSvc.Finish
	certificate.Restart = app.LifecycleRestartChan
}

// BindDatabase 在数据库初始化后把引擎注入已装配的仓储与 MFA 服务。
func BindDatabase(engine *xorm.Engine) {
	if persistenceRepo != nil {
		persistenceRepo.SetEngine(engine)
	}
	if mfaService != nil {
		mfaService.SetEngine(engine)
	}
}
