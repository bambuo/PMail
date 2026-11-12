package app

import (
	"encoding/json"
	stderrors "errors"
	"strings"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/async"
	"github.com/Jinnrry/pmail/pkg/context"
	smtp "github.com/emersion/go-smtp"
	log "github.com/sirupsen/logrus"
)

const DeliveryPendingError = "delivery result pending; outcome unknown after interruption"

type smtpOutgoingSender func(*context.Context, *parsemail.Email) (error, map[string]error)

func (s *MailSubmission) DeliverOutgoingEmail(email *parsemail.Email, sender smtpOutgoingSender) error {
	ctx := s.Ctx
	_, saved, err := s.Service.Save(ctx, email.Size, email, ctx.UserID, int(consts.EmailTypeSend), nil, true, true)
	if err != nil {
		log.WithContext(ctx).Errorf("CRITICAL audit_event=initial_persist_failed direction=outbound msg_id=%q error=%v", email.MsgID, err)
		return smtpLocalPersistenceSMTPError()
	}

	deliveryErr, domainErrors := sender(ctx, email)
	s.runSendAfterHooks(ctx, email, domainErrors)

	targetStatus := consts.EmailStatusSent
	var errorText any
	if deliveryErr != nil {
		targetStatus = consts.EmailStatusFail
		errorText = deliveryErr.Error()
	}

	if err = s.Service.audit.PersistDeliveryResult(ctx, saved.Id, targetStatus, errorText); err != nil {
		log.WithContext(ctx).Errorf(
			"CRITICAL audit_event=delivery_result_persist_failed email_id=%d msg_id=%q target_status=%d downstream_accepted=%t error=%v",
			saved.Id, saved.MsgID, targetStatus, deliveryErr == nil, err,
		)
	}

	// If the remote MX accepted DATA, returning a local 451 here would cause a
	// duplicate delivery. The persistence failure is retained as pending state
	// and a CRITICAL log; the SMTP result must still reflect the remote result.
	return s.Service.deliveryError(deliveryErr, domainErrors)
}

func (s *MailSubmission) runSendAfterHooks(ctx *context.Context, email *parsemail.Email, domainErrors map[string]error) {
	process := async.New(ctx)
	for _, hook := range s.Service.hooks() {
		if hook == nil {
			continue
		}
		process.WaitProcess(func(value any) {
			value.(contract.EmailHook).SendAfter(ctx, email, domainErrors)
		}, hook)
	}
	process.Wait()
}

func smtpLocalPersistenceSMTPError() error {
	return &smtp.SMTPError{
		Code:         451,
		EnhancedCode: smtp.EnhancedCode{4, 3, 0},
		Message:      "Temporary local persistence failure",
	}
}

func (s *EmailService) resolveIncomingUsers(ctx *context.Context, email *parsemail.Email, emailType int, reallyTo []string, SPFStatus, dkimStatus bool) ([]*models.User, bool, error) {
	if emailType != int(consts.EmailTypeReceive) {
		return nil, false, nil
	}

	accounts := smtpIncomingAccounts(email, reallyTo)
	var users []*models.User
	if len(accounts) > 0 {
		if err := s.audit.FindRecipientAccounts(accounts, &users); err != nil {
			return nil, false, err
		}
	}
	if len(users) > 0 {
		return users, false, nil
	}

	if domain.ShouldDropUnknownRecipient(SMTPDropUnknownRecipientEmails, config.Instance.SpamFilterLevel, SPFStatus, dkimStatus) {
		log.WithContext(ctx).Infoln("垃圾邮件，拒信")
		log.WithContext(ctx).Infof("收件人不存在且DKIM验证失败，丢弃邮件: %s -> %v", email.From.EmailAddress, accounts)
		return nil, true, nil
	}

	log.WithContext(ctx).Infof("收件人不存在但DKIM验证通过，转交管理员: %s -> %v", email.From.EmailAddress, accounts)
	if err := s.users.FindAdministrators(&users); err != nil {
		return nil, false, err
	}
	if len(users) == 0 {
		return nil, false, stderrors.New("no administrator available for unknown recipient")
	}
	return users, false, nil
}

func smtpIncomingAccounts(email *parsemail.Email, reallyTo []string) []string {
	var accounts []string
	if len(reallyTo) > 0 {
		for _, recipient := range reallyTo {
			account := parsemail.BuilderUser(recipient)
			if account == nil {
				continue
			}
			name, domain := account.GetDomainAccount()
			if array.InArray(domain, config.Instance.Domains) && name != "" {
				accounts = append(accounts, strings.ToLower(name))
			}
		}
		return accounts
	}
	for _, recipients := range [][]*parsemail.User{email.To, email.Cc, email.Bcc} {
		for _, user := range recipients {
			account, _ := user.GetDomainAccount()
			if account != "" {
				accounts = append(accounts, strings.ToLower(account))
			}
		}
	}
	return accounts
}

func smtpBoolInt8(value bool) int8 {
	if value {
		return 1
	}
	return 0
}

func smtpJson2string(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
