package api

import (
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
)

const smtpDeliveryPendingError = app.DeliveryPendingError

func bindTestDependencies() {
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
		NotifyMailbox:       NewIMAPIdleNotifier(app.GroupSvc).Notify,
		CheckSPF:            infra.CheckSPF,
		Hooks:               infra.PluginAllHooks,
		DeliveryError:       DeliveryResultError,
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
}
