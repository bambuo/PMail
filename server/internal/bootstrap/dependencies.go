package bootstrap

import (
	"github.com/Jinnrry/pmail/internal/api"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/pkg/context"
)

func BindDependencies() {
	repo := infra.PersistenceRepository{}

	app.UserSvc = app.NewUserService(repo)
	app.EmailQuerySvc = app.NewEmailQueryService(repo, repo)
	app.RuleSvc = app.NewRuleService(repo, repo)
	app.GroupSvc = app.NewGroupService(repo, repo)
	app.EmailSvc = app.NewEmailService(app.EmailServiceDeps{
		Emails:              repo,
		Users:               repo,
		Audit:               repo,
		OpenDeletionSession: infra.NewEmailDeletionSession,
		NotifyMailbox:       api.NewIMAPIdleNotifier(app.GroupSvc).Notify,
		CheckSPF:            infra.CheckSPF,
		Hooks:               infra.PluginAllHooks,
		DeliveryError:       api.DeliveryResultError,
		Rules:               app.RuleSvc,
	})
	app.SetupSvc = app.NewSetupService(repo, infra.InitDatabase, infra.GenerateDKIM)
	app.PluginSvc = app.NewPluginService(app.PluginPorts{
		List:      infra.PluginManageList,
		Approve:   infra.PluginManageApprove,
		Enable:    infra.PluginManageEnable,
		Disable:   infra.PluginManageDisable,
		Reject:    infra.PluginManageReject,
		Reload:    infra.PluginManageReload,
		Install:   infra.PluginManageInstall,
		HookNames: infra.PluginHookNames,
		GetHook:   infra.PluginGetHook,
	})
	app.MFASvc = app.NewMFAService(app.MFAPorts{
		Setup:           infra.MFASetup,
		Confirm:         infra.MFAConfirm,
		Disable:         infra.MFADisable,
		Reset:           infra.MFAReset,
		Status:          infra.MFAStatus,
		CreateChallenge: infra.MFACreateChallenge,
		VerifyChallenge: infra.MFAVerifyChallenge,
	})

	app.GetCertificateType = infra.CertificateGetSSL
	app.SetCertificate = infra.CertificateSetSSL
	app.GenerateCertificate = infra.CertificateGenSSL
	app.HTTPChallenge = func(token string) (string, bool) {
		info, ok := infra.CertificateGetHttpChallengeInstance().AuthInfo[token]
		if !ok {
			return "", false
		}
		return info.KeyAuth, true
	}
	app.GetDNSChallengeRecords = func(ctx *context.Context) []*app.DNSChallengeRecord {
		records := infra.CertificateGetDnsChallengeInstance().GetDNSSettings(ctx)
		out := make([]*app.DNSChallengeRecord, 0, len(records))
		for _, r := range records {
			out = append(out, &app.DNSChallengeRecord{Type: r.Type, Host: r.Host, Value: r.Value, TTL: r.TTL, Tips: r.Tips})
		}
		return out
	}
	infra.CertificateSetupFinished = app.SetupSvc.Finish
	infra.CertificateRestart = app.LifecycleRestartChan
}
