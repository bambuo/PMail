package api

import (
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
)

const smtpDeliveryPendingError = app.DeliveryPendingError

func bindTestDependencies() {
	app.Persistence = infra.PersistenceRepository{}
	app.OpenEmailDeletionSession = infra.NewEmailDeletionSession
	app.RemoveGroups = infra.RemoveGroups
	app.InitializeDatabase = infra.InitDatabase
	app.GenerateDKIM = infra.GenerateDKIM
	app.AllHooks = infra.PluginAllHooks
	app.HookNames = infra.PluginHookNames
	app.GetHook = infra.PluginGetHook
	app.CheckSPF = infra.CheckSPF
	app.PersistInitialAudit = infra.PersistInitialAudit
	app.PersistDeliveryResult = infra.PersistDeliveryResult
	app.FindRecipientAccounts = infra.FindRecipientAccounts
	app.DeliveryResultError = DeliveryResultError
	app.NotifyMailbox = IMAPIdleNotice
	app.QueryEmailList = infra.QueryEmailList
	app.QueryMailboxStat = infra.MailboxStat
	app.QueryUIDList = infra.EmailListGetUEListByUID
	app.QueryMailboxEmails = infra.EmailListGetEmailListByGroup
	app.QueryUserEmailRelations = infra.EmailDetailFindUE
}
