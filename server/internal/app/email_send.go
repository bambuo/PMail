package app

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/hooks/framework"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/async"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/send"
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

func SubmitEmail(ctx *context.Context, reqData SendEmailInput) error {
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
	for _, hook := range AllHooks() {
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

	_, err := Persistence.InsertOutgoingEmail(&modelEmail)

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
		for _, hook := range AllHooks() {
			if hook == nil {
				continue
			}
			as2.WaitProcess(func(hk any) {
				hk.(framework.EmailHook).SendAfter(ctx, e, sendErr)
			}, hook)
		}
		as2.Wait()
		log.WithContext(ctx).Debugf("插件执行--SendAfter")

		if err != nil {
			errMsg = err.Error()
			_, err := Persistence.RecordOutgoingFailure(ctx, errMsg, modelEmail.Id)
			if err != nil {
				log.WithContext(ctx).Errorf("sql Error :%+v", err)
			}

			ue := models.UserEmail{
				UserID:  ctx.UserID,
				EmailID: modelEmail.Id,
				Status:  2,
				IsRead:  1,
			}
			Persistence.InsertFailedEmailOwner(&ue)

		} else {
			_, err := Persistence.RecordOutgoingSuccess(ctx, modelEmail.Id)
			if err != nil {
				log.WithContext(ctx).Errorf("sql Error :%+v", err)
			}

			ue := models.UserEmail{
				UserID:  ctx.UserID,
				EmailID: modelEmail.Id,
				Status:  1,
				IsRead:  1,
			}
			Persistence.InsertFailedEmailOwner(&ue)

		}

	}, nil)

	return nil
}

func emailHTTPJson2string(d any) string {
	by, _ := json.Marshal(d)
	return string(by)
}
