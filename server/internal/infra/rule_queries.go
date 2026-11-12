package infra

import (
	"database/sql"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/spf13/cast"
)

func (PersistenceRepository) ListUserRules(userID int, rules *[]*models.Rule) error {
	return Database.Where("user_id=?", userID).Decr("sort").Find(rules)
}
func (PersistenceRepository) MarkRuleEmailRead(emailID int64, userID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("is_read").Update(map[string]interface {
	}{"is_read": 1})
}
func (PersistenceRepository) MarkRuleEmailDeleted(emailID int64, userID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("status").Update(map[string]interface {
	}{"status": consts.EmailStatusDel})
}
func (PersistenceRepository) FindLocalForwardOwnership(emailID int64, userID int, ue *models.UserEmail) (bool, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Get(ue)
}
func (PersistenceRepository) InsertLocalForwardOwnership(userID int, emailID int64, status int) (int64, error) {
	return Database.Insert(&models.UserEmail{UserID: userID, EmailID: cast.ToInt(emailID), Status: cast.ToInt8(status)})
}
func (PersistenceRepository) SetEmailReceiveType(emailID int64) (int64, error) {
	return Database.Table(&models.Email{}).Where("id=?", emailID).Cols("type").Update(map[string]interface {
	}{"type": consts.EmailTypeReceive})
}
func (PersistenceRepository) ResetEmailOwnerMailbox(emailID int64, userID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": 0})
}
func (PersistenceRepository) SetEmailSendType(emailID int64) (int64, error) {
	return Database.Table(&models.Email{}).Where("id=?", emailID).Cols("type").Update(map[string]interface {
	}{"type": consts.EmailTypeSend})
}
func (PersistenceRepository) SetOwnerDraftMailbox(emailID int64, userID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": consts.EmailStatusDrafts})
}
func (PersistenceRepository) SetOwnerDeletedMailbox(emailID int64, userID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": consts.EmailStatusDel})
}
func (PersistenceRepository) SetOwnerJunkMailbox(emailID int64, userID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": consts.EmailStatusJunk})
}
func (PersistenceRepository) SetOwnerCustomGroup(emailID int64, userID int, groupID int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Cols("group_id").Update(map[string]interface {
	}{"group_id": groupID})
}
func (PersistenceRepository) DeleteUserRule(ctx *context.Context, ruleID int, userID int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(ctx, "delete from rule where id =? and user_id =?"), ruleID, userID)
}
