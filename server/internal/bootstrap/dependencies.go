package bootstrap

import (
	"github.com/Jinnrry/pmail/internal/api"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/pkg/context"
)

func BindDependencies() {
	app.NotifyMailbox = api.IMAPIdleNotice
	app.DeliveryResultError = api.DeliveryResultError
	app.CheckSPF = infra.CheckSPF
	app.PersistInitialAudit = infra.PersistInitialAudit
	app.PersistDeliveryResult = infra.PersistDeliveryResult
	app.FindRecipientAccounts = infra.FindRecipientAccounts
	app.Persistence = infra.PersistenceRepository{}
	app.QueryEmailList = infra.QueryEmailList
	app.QueryMailboxStat = infra.MailboxStat
	app.QueryUIDList = infra.EmailListGetUEListByUID
	app.QueryMailboxEmails = infra.EmailListGetEmailListByGroup
	app.QueryUserEmailRelations = infra.EmailDetailFindUE
	app.OpenEmailDeletionSession = infra.NewEmailDeletionSession
	app.RemoveGroups = infra.RemoveGroups
	app.InitializeDatabase = infra.InitDatabase
	app.GenerateDKIM = infra.GenerateDKIM
	app.AllHooks = infra.PluginAllHooks
	app.HookNames = infra.PluginHookNames
	app.GetHook = infra.PluginGetHook
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
	infra.CertificateSetupFinished = app.SetupFinish
	infra.CertificateRestart = app.LifecycleRestartChan
}
