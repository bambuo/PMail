package infra

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

func (PersistenceRepository) FindEmailByID(id int, email *models.Email) (bool, error) {
	return Database.ID(id).Get(email)
}
func (PersistenceRepository) FindEmailOwnership(emailID int, userID int, owners *[]models.UserEmail) error {
	return Database.Table(&models.UserEmail{}).Where("email_id = ? and user_id = ?", emailID, userID).Find(owners)
}
func (PersistenceRepository) FindUserEmail(emailID int, userID int, ue *models.UserEmail) (bool, error) {
	return Database.Where("email_id = ? AND user_id = ?", emailID, userID).Get(ue)
}
func (PersistenceRepository) FindEmailDetail(id int, detail *response.EmailResponseData) (bool, error) {
	return Database.Select("*,1 as is_read").Table("email").Where("id=?", id).Get(detail)
}
func (PersistenceRepository) MarkUserEmailRead(ueID int, ue *models.UserEmail) (int64, error) {
	return Database.Where("id=?", ueID).Update(ue)
}
func (PersistenceRepository) SetEmailRead(emailID int, userID int, ue *models.UserEmail) (int64, error) {
	return Database.Where("email_id = ? and user_id=?", emailID, userID).Cols("is_read").Update(ue)
}
func (PersistenceRepository) FindUserEmailsByUID(ueIDs []int, ues *[]models.UserEmail) error {
	return Database.Where(builder.Eq{"id": ueIDs}).Find(ues)
}
func (PersistenceRepository) SelectInboxUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return Database.SQL(sqlText, userID, 0).Find(ues)
}
func (PersistenceRepository) SelectSentUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return Database.SQL(sqlText, userID, 1).Find(ues)
}
func (PersistenceRepository) SelectDraftUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return Database.SQL(sqlText, userID, 4).Find(ues)
}
func (PersistenceRepository) SelectDeletedUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return Database.SQL(sqlText, userID, 3).Find(ues)
}
func (PersistenceRepository) SelectJunkUserEmails(sqlText string, userID int, ues *[]models.UserEmail) error {
	return Database.SQL(sqlText, userID, 5).Find(ues)
}
func (PersistenceRepository) FindUserGroupByName(userID int, name string, group *models.Group) (bool, error) {
	return Database.Table("group").Where("user_id=? and name=?", userID, name).Get(group)
}
func (PersistenceRepository) SelectGroupUserEmails(serialNumbers []int, ues *[]models.UserEmail, userID int, groupID int) error {
	return Database.SQL(fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and group_id = ?)) a WHERE serial_number in (%s)", array.Join(serialNumbers, ","))).Find(ues, userID, groupID)
}
func (PersistenceRepository) FindEmailsByID(ids []int, emails *[]models.Email) error {
	return Database.Table("email").In("id", ids).Find(emails)
}
func (PersistenceRepository) SelectEmailList(sqlText string, args []any, emails *[]*response.EmailResponseData) error {
	return Database.SQL(sqlText, args...).Find(emails)
}
func (PersistenceRepository) CountEmailList(sqlText string, args []any, count *int64) (bool, error) {
	return Database.SQL(sqlText, args...).Get(count)
}
func (PersistenceRepository) SelectMailboxStat(sqlText string, userID int, stat *domain.EmailStatRow) (bool, error) {
	return Database.SQL(sqlText, userID).Get(stat)
}
func (PersistenceRepository) SelectMailboxUIDs(sqlText string, args []any, ues *[]*response.UserEmailUIDData) error {
	return Database.SQL(sqlText, args...).Find(ues)
}
func (PersistenceRepository) SelectInboxUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return Database.SQL(sqlText, userID, 0).Find(ues)
}
func (PersistenceRepository) SelectSentUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return Database.SQL(sqlText, userID, 1).Find(ues)
}
func (PersistenceRepository) SelectDraftUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return Database.SQL(sqlText, userID, 4).Find(ues)
}
func (PersistenceRepository) SelectDeletedUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return Database.SQL(sqlText, userID, 3).Find(ues)
}
func (PersistenceRepository) SelectJunkUIDEmails(sqlText string, userID int, ues *[]*response.UserEmailUIDData) error {
	return Database.SQL(sqlText, userID, 5).Find(ues)
}
func (PersistenceRepository) SelectGroupUIDs(serialNumbers []int, ues *[]*response.UserEmailUIDData, userID int, groupID int) error {
	return Database.SQL(fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and group_id = ?)) a WHERE serial_number in (%s)", array.Join(serialNumbers, ","))).Find(ues, userID, groupID)
}
func (PersistenceRepository) FindEmailResponsesByID(ids []int, emails *[]*response.EmailResponseData) error {
	return Database.Table("email").Select("*").Where(builder.Eq{"id": ids}).Find(emails)
}
func (PersistenceRepository) InsertOutgoingEmail(email *models.Email) (int64, error) {
	return Database.Insert(email)
}
func (PersistenceRepository) RecordOutgoingFailure(ctx *context.Context, errMsg string, emailID int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(ctx, "update email set status =2 ,error=? where id = ? "), errMsg, emailID)
}
func (PersistenceRepository) InsertFailedEmailOwner(ue *models.UserEmail) (int64, error) {
	return Database.Insert(ue)
}
func (PersistenceRepository) RecordOutgoingSuccess(ctx *context.Context, emailID int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(ctx, "update email set status =1  where id = ? "), emailID)
}
func (PersistenceRepository) UpdateUserRule(ctx *context.Context, name string, value string, action int, params string, sort int, ruleID int, userID int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(ctx, "update rule set name=? ,value = ? ,action = ?,params = ?,sort = ? where id = ? and user_id = ?"), name, value, action, params, sort, ruleID, userID)
}
func (PersistenceRepository) InsertUserRule(ctx *context.Context, name string, value string, userID int, action int, params string, sort int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(ctx, "insert into rule (name,value,user_id,action,params,sort) values (?,?,?,?,?,?)"), name, value, userID, action, params, sort)
}
func (PersistenceRepository) FindRecentUserEmails(userID int, since time.Time, ues *[]models.UserEmail) error {
	return Database.Table("user_email").Where("user_id=? and create >=?", userID, since).Find(ues)
}
func (PersistenceRepository) FindAdministrators(admins *[]*models.User) error {
	return Database.Table(&models.User{}).Where("is_admin=1").Find(admins)
}
func (PersistenceRepository) ListSavedEmailOwners(emailID int64, owners *[]*models.UserEmail) error {
	return Database.Table(&models.UserEmail{}).Where("email_id=?", emailID).Find(owners)
}
