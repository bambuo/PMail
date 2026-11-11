package infra

import (
	"database/sql"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/spf13/cast"
)

func (PersistenceRepository) ListUserRules(p0 int, p1 *[]*models.Rule) error {
	return Database.Where("user_id=?", p0).Decr("sort").Find(p1)
}
func (PersistenceRepository) MarkRuleEmailRead(p0 int64, p1 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("is_read").Update(map[string]interface {
	}{"is_read": 1})
}
func (PersistenceRepository) MarkRuleEmailDeleted(p0 int64, p1 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("status").Update(map[string]interface {
	}{"status": consts.EmailStatusDel})
}
func (PersistenceRepository) FindLocalForwardOwnership(p0 int64, p1 int, p2 *models.UserEmail) (bool, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Get(p2)
}
func (PersistenceRepository) InsertLocalForwardOwnership(p0 int, p1 int64, p2 int) (int64, error) {
	return Database.Insert(&models.UserEmail{UserID: p0, EmailID: cast.ToInt(p1), Status: cast.ToInt8(p2)})
}
func (PersistenceRepository) SetEmailReceiveType(p0 int64) (int64, error) {
	return Database.Table(&models.Email{}).Where("id=?", p0).Cols("type").Update(map[string]interface {
	}{"type": consts.EmailTypeReceive})
}
func (PersistenceRepository) ResetEmailOwnerMailbox(p0 int64, p1 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": 0})
}
func (PersistenceRepository) SetEmailSendType(p0 int64) (int64, error) {
	return Database.Table(&models.Email{}).Where("id=?", p0).Cols("type").Update(map[string]interface {
	}{"type": consts.EmailTypeSend})
}
func (PersistenceRepository) SetOwnerDraftMailbox(p0 int64, p1 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": consts.EmailStatusDrafts})
}
func (PersistenceRepository) SetOwnerDeletedMailbox(p0 int64, p1 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": consts.EmailStatusDel})
}
func (PersistenceRepository) SetOwnerJunkMailbox(p0 int64, p1 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("group_id", "status").Update(map[string]interface {
	}{"group_id": 0, "status": consts.EmailStatusJunk})
}
func (PersistenceRepository) SetOwnerCustomGroup(p0 int64, p1 int, p2 int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", p0, p1).Cols("group_id").Update(map[string]interface {
	}{"group_id": p2})
}
func (PersistenceRepository) DeleteUserRule(p0 *context.Context, p1 int, p2 int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(p0, "delete from rule where id =? and user_id =?"), p1, p2)
}
