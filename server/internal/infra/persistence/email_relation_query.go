package persistence

import (
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/internal/app"

	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

func (r PersistenceRepository) EmailDetailFindUE(ctx *context.Context, groupName string, req app.EmailListImapListReq, uid bool) []models.UserEmail {
	var ue []models.UserEmail
	if uid {
		err := r.FindUserEmailsByUID(req.UidList, &ue)
		if err != nil {
			log.WithContext(ctx).Errorf("SQL error:%+v", err)
		}
		return ue
	} else {
		sql := fmt.Sprintf("SELECT id,email_id, is_read from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and status = ?)) a WHERE serial_number in (%s)",
			array.Join(req.UidList, ","),
		)
		switch groupName {
		case "INBOX":
			r.SelectInboxUserEmails(sql, ctx.UserID, &ue)
		case "Sent Messages":
			r.SelectSentUserEmails(sql, ctx.UserID, &ue)
		case "Drafts":
			r.SelectDraftUserEmails(sql, ctx.UserID, &ue)
		case "Deleted Messages":
			r.SelectDeletedUserEmails(sql, ctx.UserID, &ue)
		case "Junk":
			r.SelectJunkUserEmails(sql, ctx.UserID, &ue)
		default:
			groupNames := strings.Split(groupName, "/")
			groupName = groupNames[len(groupNames)-1]

			var group models.Group
			r.FindUserGroupByName(ctx.UserID, groupName, &group)
			if group.ID == 0 {
				return nil
			}
			r.SelectGroupUserEmails(req.UidList, &ue, ctx.UserID, group.ID)
		}

		return ue

	}
}
