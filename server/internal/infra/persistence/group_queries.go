package persistence

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

func (r *PersistenceRepository) FindGroupByName(name string, userID int, group *models.Group) (bool, error) {
	return r.db().Table("group").Where("name = ? and user_id = ?", name, userID).Get(group)
}
func (r *PersistenceRepository) InsertGroup(group *models.Group) (int64, error) {
	return r.db().Insert(group)
}
func (r *PersistenceRepository) UpdateGroup(id int, group *models.Group) (int64, error) {
	return r.db().ID(id).Update(group)
}
func (r *PersistenceRepository) FindGroupByPath(fullPath string, userID int, group *models.Group) (bool, error) {
	return r.db().Table("group").Where("full_path = ? and user_id = ?", fullPath, userID).Get(group)
}
func (r *PersistenceRepository) FindChildGroupIDs(parentID int, userID int, rows *[]domain.GroupIDRow) error {
	return r.db().Table("group").Where("parent_id=? and user_id=?", parentID, userID).Find(rows)
}
func (r *PersistenceRepository) MoveEmailsToGroup(ctx *context.Context, emailIDs []int, groupID int, userID int) (sql.Result, error) {
	return r.db().Exec(DatabaseWithContext(ctx, fmt.Sprintf("update user_email set group_id=? where email_id in (%s) and user_id =?", array.Join(emailIDs, ","))), groupID, userID)
}
func (r *PersistenceRepository) FindChildGroups(parentID int, userID int, groups *[]*models.Group) error {
	return r.db().Table("group").Where("parent_id=? and user_id=?", parentID, userID).Find(groups)
}
func (r *PersistenceRepository) ListUserGroups(userID int, groups *[]*models.Group) error {
	return r.db().Table("group").Where("user_id=?", userID).Find(groups)
}
func (r *PersistenceRepository) FindGroupsByParent(parentID int, groups *[]*models.Group) error {
	return r.db().Table("group").Where("parent_id=?", parentID).Find(groups)
}
func (r *PersistenceRepository) FindGroupByID(id int, group *models.Group) (bool, error) {
	return r.db().Table("group").Where("id=?", id).Get(group)
}
func (r *PersistenceRepository) CountGroupEmails(groupID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("group_id=?", groupID).Get(count)
}
func (r *PersistenceRepository) FindGroupLastUID(groupID int, uid *int) (bool, error) {
	return r.db().Table("user_email").Select("id").Where("group_id=?", groupID).OrderBy("id desc").Get(uid)
}
func (r *PersistenceRepository) CountGroupUnreadEmails(groupID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("group_id=? and is_read=0", groupID).Get(count)
}
func (r *PersistenceRepository) FindLastInboxUID(userID int, uid *int) (bool, error) {
	return r.db().Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 0", userID).OrderBy("id desc").Get(uid)
}
func (r *PersistenceRepository) FindLastSentUID(userID int, uid *int) (bool, error) {
	return r.db().Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 1", userID).OrderBy("id desc").Get(uid)
}
func (r *PersistenceRepository) FindLastDraftUID(userID int, uid *int) (bool, error) {
	return r.db().Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 4", userID).OrderBy("id desc").Get(uid)
}
func (r *PersistenceRepository) FindLastDeletedUID(userID int, uid *int) (bool, error) {
	return r.db().Table("user_email").Select("id").Where("user_id=? and status = 3", userID).OrderBy("id desc").Get(uid)
}
func (r *PersistenceRepository) FindLastJunkUID(userID int, uid *int) (bool, error) {
	return r.db().Table("user_email").Select("id").Where("user_id=? and group_id=0 and status = 5", userID).OrderBy("id desc").Get(uid)
}
func (r *PersistenceRepository) CountUnreadInboxEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=0 and is_read=0", userID).Get(count)
}
func (r *PersistenceRepository) CountInboxEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=0", userID).Get(count)
}
func (r *PersistenceRepository) CountSentEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=1", userID).Get(count)
}
func (r *PersistenceRepository) CountUnreadDraftEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=4 and is_read=0", userID).Get(count)
}
func (r *PersistenceRepository) CountDraftEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=4", userID).Get(count)
}
func (r *PersistenceRepository) CountUnreadDeletedEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=3 and is_read=0", userID).Get(count)
}
func (r *PersistenceRepository) CountDeletedEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=3", userID).Get(count)
}
func (r *PersistenceRepository) CountUnreadJunkEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=5 and is_read=0", userID).Get(count)
}
func (r *PersistenceRepository) CountJunkEmails(userID int, count *int) (bool, error) {
	return r.db().Table("user_email").Select("count(1)").Where("user_id=? and status=5", userID).Get(count)
}
func (r *PersistenceRepository) MoveEmailsToInbox(userID int, emailIDs []int) (int64, error) {
	return r.db().Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailTypeReceive, "group_id": 0})
}
func (r *PersistenceRepository) MoveEmailsToSent(userID int, emailIDs []int) (int64, error) {
	return r.db().Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailStatusSent, "group_id": 0})
}
func (r *PersistenceRepository) MoveEmailsToDraft(userID int, emailIDs []int) (int64, error) {
	return r.db().Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailStatusDrafts, "group_id": 0})
}
func (r *PersistenceRepository) MoveEmailsToJunk(userID int, emailIDs []int) (int64, error) {
	return r.db().Table(&models.UserEmail{}).Where(builder.Eq{"user_id": userID, "email_id": emailIDs}).Update(map[string]interface {
	}{"status": consts.EmailStatusJunk, "group_id": 0})
}
func (r *PersistenceRepository) FindGroupsByPathPattern(userID int, pattern string, groups *[]*models.Group) error {
	return r.db().Table("group").Where("user_id=? and full_path like ?", userID, pattern).Find(groups)
}
