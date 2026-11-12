package app

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	stderrors "errors"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/async"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/send"
	smtp "github.com/emersion/go-smtp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

type SendEmailInput struct {
	ReplyTo     []MailAddressInput    `json:"reply_to"`
	From        MailAddressInput      `json:"from"`
	To          []MailAddressInput    `json:"to"`
	Bcc         []MailAddressInput    `json:"bcc"`
	Cc          []MailAddressInput    `json:"cc"`
	Subject     string                `json:"subject"`
	Text        string                `json:"text"`   // Plaintext message (optional)
	HTML        string                `json:"html"`   // Html message (optional)
	Sender      MailAddressInput      `json:"sender"` // override From as SMTP envelope sender (optional)
	ReadReceipt []string              `json:"read_receipt"`
	Attachments []MailAttachmentInput `json:"attrs"`
}

type MailAddressInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type MailAttachmentInput struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

func (s *EmailService) Submit(ctx *context.Context, reqData SendEmailInput) error {
	if reqData.From.Email != "" {
		infos := strings.Split(reqData.From.Email, "@")
		if len(infos) != 2 || !array.InArray(infos[1], config.Instance.Domains) {
			return NewUseCaseError(response.ParamsError, "params error", "")
		}
		if !domain.SenderAllowed(ctx.IsAdmin, ctx.UserAccount, infos[0]) {
			return NewUseCaseError(response.ParamsError, "params error", "")
		}

	}

	if reqData.From.Email == "" {
		reqData.From.Email = ctx.UserAccount + "@" + config.Instance.Domain
	}

	if reqData.From.Email == "" {
		return NewUseCaseError(response.ParamsError, "发件人必填", "发件人必填")
	}

	if reqData.From.Name == "" {
		reqData.From.Name = ctx.UserName
	}

	if reqData.Subject == "" {
		return NewUseCaseError(response.ParamsError, "邮件标题必填", "邮件标题必填")
	}

	if len(reqData.To) <= 0 {
		return NewUseCaseError(response.ParamsError, "收件人必填", "收件人必填")
	}

	e := &parsemail.Email{}

	for _, to := range reqData.To {
		e.To = append(e.To, &parsemail.User{
			Name:         to.Name,
			EmailAddress: to.Email,
		})
	}

	for _, bcc := range reqData.Bcc {
		e.Bcc = append(e.Bcc, &parsemail.User{
			Name:         bcc.Name,
			EmailAddress: bcc.Email,
		})
	}

	for _, cc := range reqData.Cc {
		e.Cc = append(e.Cc, &parsemail.User{
			Name:         cc.Name,
			EmailAddress: cc.Email,
		})
	}

	e.From = &parsemail.User{
		Name:         reqData.From.Name,
		EmailAddress: reqData.From.Email,
	}
	if reqData.Sender.Email != "" {
		e.Sender = &parsemail.User{
			Name:         reqData.Sender.Name,
			EmailAddress: reqData.Sender.Email,
		}
	} else {
		e.Sender = &parsemail.User{
			Name:         reqData.From.Name,
			EmailAddress: reqData.From.Email,
		}
	}

	e.Text = []byte(reqData.Text)
	e.HTML = []byte(reqData.HTML)
	e.Subject = reqData.Subject
	for _, att := range reqData.Attachments {
		att.Data = strings.TrimPrefix(att.Data, "data:")
		infos := strings.Split(att.Data, ";")
		contentType := infos[0]
		content := strings.TrimPrefix(infos[1], "base64,")
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			log.WithContext(ctx).Errorf("附件解码错误！%v", err)
			return NewUseCaseError(response.ParamsError, TextGetText(ctx.Lang, "att_err"), err.Error())
		}
		e.Attachments = append(e.Attachments, &parsemail.Attachment{
			Filename:    att.Name,
			ContentType: contentType,
			Content:     decoded,
		})

	}

	log.WithContext(ctx).Debugf("插件执行--SendBefore")
	for _, hook := range s.hooks() {
		if hook == nil {
			continue
		}
		hook.SendBefore(ctx, e)
	}
	log.WithContext(ctx).Debugf("插件执行--SendBefore End")

	modelEmail := models.Email{
		Type:         1,
		Subject:      e.Subject,
		ReplyTo:      emailHTTPJson2string(e.ReplyTo),
		FromName:     e.From.Name,
		FromAddress:  e.From.EmailAddress,
		To:           emailHTTPJson2string(e.To),
		Bcc:          emailHTTPJson2string(e.Bcc),
		Cc:           emailHTTPJson2string(e.Cc),
		Text:         sql.NullString{String: string(e.Text), Valid: true},
		Html:         sql.NullString{String: string(e.HTML), Valid: true},
		Sender:       emailHTTPJson2string(e.Sender),
		Attachments:  emailHTTPJson2string(e.Attachments),
		SPFCheck:     1,
		DKIMCheck:    1,
		SendUserID:   ctx.UserID,
		SendDate:     time.Now(),
		CronSendTime: time.Now(),
		Status:       1,
		CreateTime:   time.Now(),
		MsgID:        parsemail.GenerateMsgID(config.Instance.Domain),
	}

	_, err := s.emails.InsertOutgoingEmail(&modelEmail)

	if err != nil || modelEmail.Id <= 0 {
		log.Println("insert error:", err.Error())
		return NewUseCaseError(response.ServerError, TextGetText(ctx.Lang, "send_fail"), err.Error())
	}

	e.MessageId = cast.ToInt64(modelEmail.Id)
	e.MsgID = modelEmail.MsgID

	async.New(ctx).Process(func(p any) {
		errMsg := ""
		err, sendErr := send.Send(ctx, e)

		log.WithContext(ctx).Debugf("插件执行--SendAfter")

		as2 := async.New(ctx)
		for _, hook := range s.hooks() {
			if hook == nil {
				continue
			}
			as2.WaitProcess(func(hk any) {
				hk.(contract.EmailHook).SendAfter(ctx, e, sendErr)
			}, hook)
		}
		as2.Wait()
		log.WithContext(ctx).Debugf("插件执行--SendAfter")

		if err != nil {
			errMsg = err.Error()
			_, err := s.emails.RecordOutgoingFailure(ctx, errMsg, modelEmail.Id)
			if err != nil {
				log.WithContext(ctx).Errorf("sql Error :%+v", err)
			}

			ue := models.UserEmail{
				UserID:  ctx.UserID,
				EmailID: modelEmail.Id,
				Status:  2,
				IsRead:  1,
			}
			s.emails.InsertFailedEmailOwner(&ue)

		} else {
			_, err := s.emails.RecordOutgoingSuccess(ctx, modelEmail.Id)
			if err != nil {
				log.WithContext(ctx).Errorf("sql Error :%+v", err)
			}

			ue := models.UserEmail{
				UserID:  ctx.UserID,
				EmailID: modelEmail.Id,
				Status:  1,
				IsRead:  1,
			}
			s.emails.InsertFailedEmailOwner(&ue)

		}

	}, nil)

	return nil
}

func emailHTTPJson2string(d any) string {
	by, _ := json.Marshal(d)
	return string(by)
}

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
