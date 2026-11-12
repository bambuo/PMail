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

func (PersistenceRepository) FindGroupByName(name string, userID int, group *models.Group) (bool, error) {
	return Database.Table("group").Where("name = ? and user_id = ?", name, userID).Get(group)
}
func (PersistenceRepository) InsertGroup(group *models.Group) (int64, error) {
	return Database.Insert(group)
}
func (PersistenceRepository) UpdateGroup(id int, group *models.Group) (int64, error) {
	return Database.ID(id).Update(group)
}
func (PersistenceRepository) FindGroupByPath(fullPath string, userID int, group *models.Group) (bool, error) {
	return Database.Table("group").Where("full_path = ? and user_id = ?", fullPath, userID).Get(group)
}
func (PersistenceRepository) FindChildGroupIDs(parentID int, userID int, rows *[]domain.GroupIDRow) error {
	return Database.Table("group").Where("parent_id=? and user_id=?", parentID, userID).Find(rows)
}
func (PersistenceRepository) MoveEmailsToGroup(ctx *context.Context, emailIDs []int, groupID int, userID int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(ctx, fmt.Sprintf("update user_email set group_id=? where email_id in (%s) and user_id =?", array.Join(emailIDs, ","))), groupID, userID)
}
func (PersistenceRepository) FindChildGroups(parentID int, userID int, groups *[]*models.Group) error {
	return Database.Table("group").Where("parent_id=? and user_id=?", parentID, userID).Find(groups)
}
func (PersistenceRepository) ListUserGroups(userID int, groups *[]*models.Group) error {
	return Database.Table("group").Where("user_id=?", userID).Find(groups)
}
func (PersistenceRepository) FindGroupsByParent(parentID int, groups *[]*models.Group) error {
	return Database.Table("group").Where("parent_id=?", parentID).Find(groups)
}
func (PersistenceRepository) FindGroupByID(id int, group *models.Group) (bool, error) {
	return Database.Table("group").Where("id=?", id).Get(group)
}
func (PersistenceRepository) CountGroupEmails(groupID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("group_id=?", groupID).Get(count)
}
func (PersistenceRepository) FindGroupLastUID(groupID int, uid *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("group_id=?", groupID).OrderBy("id desc").Get(uid)
}
func (PersistenceRepository) CountGroupUnreadEmails(groupID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("group_id=? and is_read=0", groupID).Get(count)
}
func (PersistenceRepository) FindLastInboxUID(userID int, uid *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 0", userID).OrderBy("id desc").Get(uid)
}
func (PersistenceRepository) FindLastSentUID(userID int, uid *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 1", userID).OrderBy("id desc").Get(uid)
}
func (PersistenceRepository) FindLastDraftUID(userID int, uid *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 4", userID).OrderBy("id desc").Get(uid)
}
func (PersistenceRepository) FindLastDeletedUID(userID int, uid *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and status = 3", userID).OrderBy("id desc").Get(uid)
}
func (PersistenceRepository) FindLastJunkUID(userID int, uid *int) (bool, error) {
	return Database.Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 5", userID).OrderBy("id desc").Get(uid)
}
func (PersistenceRepository) CountUnreadInboxEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=0 and is_read=0", userID).Get(count)
}
func (PersistenceRepository) CountInboxEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=0", userID).Get(count)
}
func (PersistenceRepository) CountSentEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=1", userID).Get(count)
}
func (PersistenceRepository) CountUnreadDraftEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=4 and is_read=0", userID).Get(count)
}
func (PersistenceRepository) CountDraftEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=4", userID).Get(count)
}
func (PersistenceRepository) CountUnreadDeletedEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=3 and is_read=0", userID).Get(count)
}
func (PersistenceRepository) CountDeletedEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=3", userID).Get(count)
}
func (PersistenceRepository) CountUnreadJunkEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=5 and is_read=0", userID).Get(count)
}
func (PersistenceRepository) CountJunkEmails(userID int, count *int) (bool, error) {
	return Database.Table("user_email").Select("count(1)").Where("user_id=? and status=5", userID).Get(count)
}
func (PersistenceRepository) MoveEmailsToInbox(userID int, emailIDs []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailTypeReceive, "group_id": 0})
}
func (PersistenceRepository) MoveEmailsToSent(userID int, emailIDs []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailStatusSent, "group_id": 0})
}
func (PersistenceRepository) MoveEmailsToDraft(userID int, emailIDs []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailStatusDrafts, "group_id": 0})
}
func (PersistenceRepository) MoveEmailsToJunk(userID int, emailIDs []int) (int64, error) {
	return Database.Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailStatusJunk, "group_id": 0})
}
func (PersistenceRepository) FindGroupsByPathPattern(userID int, pattern string, groups *[]*models.Group) error {
	return Database.Table("group").Where("user_id=? and full_path like ?", userID, pattern).Find(groups)
}
