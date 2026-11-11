package app

import (
	"encoding/json"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

func AttachmentGetAttachments(ctx *context.Context, emailId int, cid string) (string, []byte) {

	// 获取邮件内容
	var email models.Email
	_, err := Persistence.FindEmailByID(emailId, &email)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL error:%+v", err)
		return "", nil
	}

	// 检查权限
	if !AuthHasAuth(ctx, &email) {
		return "", nil
	}

	var atts []parsemail.Attachment
	_ = json.Unmarshal([]byte(email.Attachments), &atts)
	for _, att := range atts {
		if att.ContentID == cid {
			return att.ContentType, att.Content
		}
	}
	return "", nil
}

func AttachmentGetAttachmentsByIndex(ctx *context.Context, emailId int, index int) (string, []byte) {

	// 获取邮件内容
	var email models.Email
	_, err := Persistence.FindEmailByID(emailId, &email)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL error:%+v", err)
		return "", nil
	}

	// 检查权限
	if !AuthHasAuth(ctx, &email) {
		return "", nil
	}

	var atts []parsemail.Attachment
	_ = json.Unmarshal([]byte(email.Attachments), &atts)

	if len(atts) > index {
		return atts[index].Filename, atts[index].Content
	}
	return "", nil
}
