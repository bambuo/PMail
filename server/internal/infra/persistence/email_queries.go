package persistence

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"xorm.io/builder"
)

func (r *PersistenceRepository) FindEmailByID(id int, email *models.Email) (bool, error) {
	return r.db().ID(id).Get(email)
}
func (r *PersistenceRepository) FindEmailOwnership(emailID int, userID int, owners *[]models.UserEmail) error {
	return r.db().Table(&models.UserEmail{}).Where("email_id = ? and user_id = ?", emailID, userID).Find(owners)
}
func (r *PersistenceRepository) FindUserEmail(emailID int, userID int, ue *models.UserEmail) (bool, error) {
	return r.db().Where("email_id = ? AND user_id = ?", emailID, userID).Get(ue)
}
func (r *PersistenceRepository) FindEmailDetail(id int, detail *response.EmailResponseData) (bool, error) {
	return r.db().Select("*,1 as is_read").Table("email").Where("id=?", id).Get(detail)
}
func (r *PersistenceRepository) MarkUserEmailRead(ueID int, ue *models.UserEmail) (int64, error) {
	return r.db().Where("id=?", ueID).Update(ue)
}
func (r *PersistenceRepository) SetEmailRead(emailID int, userID int, ue *models.UserEmail) (int64, error) {
	return r.db().Where("email_id = ? and user_id=?", emailID, userID).Cols("is_read").Update(ue)
}
func (r *PersistenceRepository) FindUserEmailsByUID(ueIDs []int, ues *[]models.UserEmail) error {
	return r.db().Where(builder.Eq{"id": ueIDs}).Find(ues)
}
func (r *PersistenceRepository) SelectInboxUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return r.db().SQL(sqlText, userID, 0).Find(ues)
}
func (r *PersistenceRepository) SelectSentUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return r.db().SQL(sqlText, userID, 1).Find(ues)
}
func (r *PersistenceRepository) SelectDraftUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return r.db().SQL(sqlText, userID, 4).Find(ues)
}
func (r *PersistenceRepository) SelectDeletedUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return r.db().SQL(sqlText, userID, 3).Find(ues)
}
func (r *PersistenceRepository) SelectJunkUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return r.db().SQL(sqlText, userID, 5).Find(ues)
}
func (r *PersistenceRepository) FindUserGroupByName(userID int, name string, group *models.Group) (bool, error) {
	return r.db().Table("group").Where("user_id=? and name=?", userID, name).Get(group)
}
func (r *PersistenceRepository) SelectGroupUserEmails(serialNumbers []int, ues *[]models.UserEmail, userID int, groupID int) error {
	return r.db().SQL(fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and group_id = ?)) a WHERE serial_number in (%s)", array.Join(serialNumbers, ","))).Find(ues, userID, groupID)
}
func (r *PersistenceRepository) FindEmailsByID(ids []int, emails *[]models.Email) error {
	return r.db().Table("email").In("id", ids).Find(emails)
}
func (r *PersistenceRepository) SelectEmailList(sqlText string, args []any, emails *[]*response.EmailResponseData) error {
	return r.db().SQL(sqlText, args...).Find(emails)
}
func (r *PersistenceRepository) CountEmailList(sqlText string, args []any, count *int64) (bool, error) {
	return r.db().SQL(sqlText, args...).Get(count)
}
func (r *PersistenceRepository) SelectMailboxStat(sqlText string, userID int, stat *domain.EmailStatRow) (bool, error) {
	return r.db().SQL(sqlText, userID).Get(stat)
}
func (r *PersistenceRepository) SelectMailboxUIDs(sqlText string, args []any, ues *[]*response.UserEmailUIDData) error {
	return r.db().SQL(sqlText, args...).Find(ues)
}
func (r *PersistenceRepository) SelectInboxUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return r.db().SQL(sqlText, userID, 0).Find(ues)
}
func (r *PersistenceRepository) SelectSentUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return r.db().SQL(sqlText, userID, 1).Find(ues)
}
func (r *PersistenceRepository) SelectDraftUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return r.db().SQL(sqlText, userID, 4).Find(ues)
}
func (r *PersistenceRepository) SelectDeletedUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return r.db().SQL(sqlText, userID, 3).Find(ues)
}
func (r *PersistenceRepository) SelectJunkUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return r.db().SQL(sqlText, userID, 5).Find(ues)
}
func (r *PersistenceRepository) SelectGroupUIDs(serialNumbers []int, ues *[]*response.UserEmailUIDData, userID int, groupID int) error {
	return r.db().SQL(fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and group_id = ?)) a WHERE serial_number in (%s)", array.Join(serialNumbers, ","))).Find(ues, userID, groupID)
}
func (r *PersistenceRepository) FindEmailResponsesByID(ids []int, emails *[]*response.EmailResponseData) error {
	return r.db().Table("email").Select("*").Where(builder.Eq{"id": ids}).Find(emails)
}
func (r *PersistenceRepository) InsertOutgoingEmail(email *models.Email) (int64, error) {
	return r.db().Insert(email)
}
func (r *PersistenceRepository) RecordOutgoingFailure(ctx *context.Context, errMsg string, emailID int) (sql.Result, error) {
	return r.db().Exec(DatabaseWithContext(ctx, "update email set status =2 ,error=? where id = ? "), errMsg, emailID)
}
func (r *PersistenceRepository) InsertFailedEmailOwner(ue *models.UserEmail) (int64, error) {
	return r.db().Insert(ue)
}
func (r *PersistenceRepository) RecordOutgoingSuccess(ctx *context.Context, emailID int) (sql.Result, error) {
	return r.db().Exec(DatabaseWithContext(ctx, "update email set status =1  where id = ? "), emailID)
}
func (r *PersistenceRepository) UpdateUserRule(ctx *context.Context, name string, value string, action int, params string, sort int, ruleID int, userID int) (sql.Result, error) {
	return r.db().Exec(DatabaseWithContext(ctx, "update rule set name=? ,value = ? ,action = ?,params = ?,sort = ? where id = ? and user_id = ?"), name, value, action, params, sort, ruleID, userID)
}
func (r *PersistenceRepository) InsertUserRule(ctx *context.Context, name string, value string, userID int, action int, params string, sort int) (sql.Result, error) {
	return r.db().Exec(DatabaseWithContext(ctx, "insert into rule (name,value,user_id,action,params,sort) values (?,?,?,?,?,?)"), name, value, userID, action, params, sort)
}
func (r *PersistenceRepository) FindRecentUserEmails(userID int, since time.Time, ues *[]models.UserEmail) error {
	return r.db().Table("user_email").Where("user_id=? and create >=?", userID, since).Find(ues)
}
func (r *PersistenceRepository) FindAdministrators(admins *[]*models.User) error {
	return r.db().Table(&models.User{}).Where("is_admin=1").Find(admins)
}
func (r *PersistenceRepository) ListSavedEmailOwners(emailID int64, owners *[]*models.UserEmail) error {
	return r.db().Table(&models.UserEmail{}).Where("email_id=?", emailID).Find(owners)
}
