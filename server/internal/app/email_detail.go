package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func EmailDetailGetEmailDetail(ctx *context.Context, id int, markRead bool) (*response.EmailResponseData, error) {
	// 先查是否是本人的邮件
	var ue models.UserEmail
	_, err := Persistence.FindUserEmail(id, ctx.UserID, &ue)
	if err != nil {
		log.Error(err)
	}
	if ue.ID == 0 && !ctx.IsAdmin {
		return nil, errors.New("Not authorized")
	}

	//获取邮件内容
	var email response.EmailResponseData
	_, err = Persistence.FindEmailDetail(id, &email)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL error:%+v", err)
		return nil, err
	}

	email.IsRead = ue.IsRead

	if markRead && ue.IsRead == 0 {
		ue.IsRead = 1
		_, err = Persistence.MarkUserEmailRead(ue.ID, &ue)
		if err != nil {
			log.WithContext(ctx).Errorf("SQL error:%+v", err)
		}
	}

	// 将内容中的cid内容替换成url
	if email.Attachments != "" {
		var atts []parsemail.Attachment
		_ = json.Unmarshal([]byte(email.Attachments), &atts)
		for _, att := range atts {
			email.Html = sql.NullString{
				String: strings.ReplaceAll(email.Html.String, fmt.Sprintf("cid:%s", att.ContentID), fmt.Sprintf("/attachments/%d/%s", id, att.ContentID)),
			}
		}
	}

	return &email, nil
}

func EmailDetailMakeRead(ctx *context.Context, emailId int, hadRead bool) {
	ue := models.UserEmail{
		UserID:  ctx.UserID,
		IsRead:  1,
		EmailID: emailId,
	}
	if !hadRead {
		ue.IsRead = 0
	}
	Persistence.SetEmailRead(emailId, ctx.UserID, &ue)
}

var QueryUserEmailRelations func(*context.Context, string, EmailListImapListReq, bool) []models.UserEmail

func EmailDetailFindUE(ctx *context.Context, name string, req EmailListImapListReq, uid bool) []models.UserEmail {
	return QueryUserEmailRelations(ctx, name, req, uid)
}
