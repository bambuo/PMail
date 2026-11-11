package infra

import (
	"database/sql"
	"fmt"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"xorm.io/builder"
)

func (PersistenceRepository) FindGroupByName(p0 string, p1 int, p2 *models.Group) (bool, error) {
	return Database.Table("group").Where("name = ? and user_id = ?", p0, p1).Get(p2)
}
func (PersistenceRepository) InsertGroup(p0 *models.Group) (int64, error) {
	return Database.Insert(p0)
}
func (PersistenceRepository) UpdateGroup(p0 int, p1 *models.Group) (int64, error) {
	return Database.ID(p0).Update(p1)
}
func (PersistenceRepository) FindGroupByPath(p0 string, p1 int, p2 *models.Group) (bool, error) {
	return Database.Table("group").Where("full_path = ? and user_id = ?", p0, p1).Get(p2)
}
func (PersistenceRepository) FindChildGroupIDs(p0 int, p1 int, p2 *[]domain.GroupIDRow) error {
	return Database.Table("group").Where("parent_id=? and user_id=?", p0, p1).Find(p2)
}
func (PersistenceRepository) MoveEmailsToGroup(p0 *context.Context, p1 []int, p2 int, p3 int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(p0, fmt.Sprintf("update user_email set group_id=? where email_id in (%s) and user_id =?", array.Join(p1, ","))), p2, p3)
}
func (PersistenceRepository) FindChildGroups(p0 int, p1 int, p2 *[]*models.Group) error {
	return Database.Table("group").Where("parent_id=? and user_id=?", p0, p1).Find(p2)
}
func (PersistenceRepository) ListUserGroups(p0 int, p1 *[]*models.Group) error {
	return Database.Table("group").Where("user_id=?", p0).Find(p1)
}
func (PersistenceRepository) FindGroupsByParent(p0 int, p1 *[]*models.Group) error {
	return Database.Table("group").Where("parent_id=?", p0).Find(p1)
}
func (PersistenceRepository) FindGroupByID(p0 int, p1 *models.Group) (bool, error) {
	return Database.Table("group").Where("id=?", p0).Get(p1)
}
func (PersistenceRepository) CountGroupEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("group_id=?", p0).Get(p1)
}
func (PersistenceRepository) FindGroupLastUID(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("group_id=?", p0).OrderBy("id desc").Get(p1)
}
func (PersistenceRepository) CountGroupUnreadEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("group_id=? and is_read=0", p0).Get(p1)
}
func (PersistenceRepository) FindLastInboxUID(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 0", p0).OrderBy("id desc").Get(p1)
}
func (PersistenceRepository) FindLastSentUID(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 1", p0).OrderBy("id desc").Get(p1)
}
func (PersistenceRepository) FindLastDraftUID(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 4", p0).OrderBy("id desc").Get(p1)
}
func (PersistenceRepository) FindLastDeletedUID(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and status = 3", p0).OrderBy("id desc").Get(p1)
}
func (PersistenceRepository) FindLastJunkUID(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 5", p0).OrderBy("id desc").Get(p1)
}
func (PersistenceRepository) CountUnreadInboxEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=0 and is_read=0", p0).Get(p1)
}
func (PersistenceRepository) CountInboxEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=0", p0).Get(p1)
}
func (PersistenceRepository) CountSentEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=1", p0).Get(p1)
}
func (PersistenceRepository) CountUnreadDraftEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=4 and is_read=0", p0).Get(p1)
}
func (PersistenceRepository) CountDraftEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=4", p0).Get(p1)
}
func (PersistenceRepository) CountUnreadDeletedEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=3 and is_read=0", p0).Get(p1)
}
func (PersistenceRepository) CountDeletedEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=3", p0).Get(p1)
}
func (PersistenceRepository) CountUnreadJunkEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=5 and is_read=0", p0).Get(p1)
}
func (PersistenceRepository) CountJunkEmails(p0 int, p1 *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=5", p0).Get(p1)
}
func (PersistenceRepository) MoveEmailsToInbox(p0 int, p1 []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": p0, "email_id": p1}).Update(map[string]interface {
	}{"status": consts.EmailTypeReceive, "group_id": 0})
}
func (PersistenceRepository) MoveEmailsToSent(p0 int, p1 []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": p0, "email_id": p1}).Update(map[string]interface {
	}{"status": consts.EmailStatusSent, "group_id": 0})
}
func (PersistenceRepository) MoveEmailsToDraft(p0 int, p1 []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": p0, "email_id": p1}).Update(map[string]interface {
	}{"status": consts.EmailStatusDrafts, "group_id": 0})
}
func (PersistenceRepository) MoveEmailsToJunk(p0 int, p1 []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": p0, "email_id": p1}).Update(map[string]interface {
	}{"status": consts.EmailStatusJunk, "group_id": 0})
}
func (PersistenceRepository) FindGroupsByPathPattern(p0 int, p1 string, p2 *[]*models.Group) error {
	return Database.Table("group").Where("user_id=? and full_path like ?", p0, p1).Find(p2)
}
