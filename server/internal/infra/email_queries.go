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

func (PersistenceRepository) FindEmailByID(p0 int, p1 *models.Email) (bool, error) {
	return Database.ID(p0).Get(p1)
}
func (PersistenceRepository) FindEmailOwnership(p0 int, p1 int, p2 *[]models.UserEmail) error {
	return Database.Table(&models.UserEmail{}).Where("email_id = ? and user_id = ?", p0, p1).Find(p2)
}
func (PersistenceRepository) FindUserEmail(p0 int, p1 int, p2 *models.UserEmail) (bool, error) {
	return Database.Where("email_id = ? AND user_id = ?", p0, p1).Get(p2)
}
func (PersistenceRepository) FindEmailDetail(p0 int, p1 *response.EmailResponseData) (bool, error) {
	return Database.Select("*,1 as is_read").Table("email").Where("id=?", p0).Get(p1)
}
func (PersistenceRepository) MarkUserEmailRead(p0 int, p1 *models.UserEmail) (int64, error) {
	return Database.Where("id=?", p0).Update(p1)
}
func (PersistenceRepository) SetEmailRead(p0 int, p1 int, p2 *models.UserEmail) (int64, error) {
	return Database.Where("email_id = ? and user_id=?", p0, p1).Cols("is_read").Update(p2)
}
func (PersistenceRepository) FindUserEmailsByUID(p0 []int, p1 *[]models.UserEmail) error {
	return Database.Where(builder.Eq{"id": p0}).Find(p1)
}
func (PersistenceRepository) SelectInboxUserEmails(p0 string, p1 int, p2 *[]models.UserEmail) error {
	return Database.SQL(p0, p1, 0).Find(p2)
}
func (PersistenceRepository) SelectSentUserEmails(p0 string, p1 int, p2 *[]models.UserEmail) error {
	return Database.SQL(p0, p1, 1).Find(p2)
}
func (PersistenceRepository) SelectDraftUserEmails(p0 string, p1 int, p2 *[]models.UserEmail) error {
	return Database.SQL(p0, p1, 4).Find(p2)
}
func (PersistenceRepository) SelectDeletedUserEmails(p0 string, p1 int, p2 *[]models.UserEmail) error {
	return Database.SQL(p0, p1, 3).Find(p2)
}
func (PersistenceRepository) SelectJunkUserEmails(p0 string, p1 int, p2 *[]models.UserEmail) error {
	return Database.SQL(p0, p1, 5).Find(p2)
}
func (PersistenceRepository) FindUserGroupByName(p0 int, p1 string, p2 *models.Group) (bool, error) {
	return Database.Table("group").Where("user_id=? and name=?", p0, p1).Get(p2)
}
func (PersistenceRepository) SelectGroupUserEmails(p0 []int, p1 *[]models.UserEmail, p2 int, p3 int) error {
	return Database.SQL(fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and group_id = ?)) a WHERE serial_number in (%s)", array.Join(p0, ","))).Find(p1, p2, p3)
}
func (PersistenceRepository) FindEmailsByID(p0 []int, p1 *[]models.Email) error {
	return Database.Table("email").In("id", p0).Find(p1)
}
func (PersistenceRepository) SelectEmailList(p0 string, p1 []any, p2 *[]*response.EmailResponseData) error {
	return Database.SQL(p0, p1...).Find(p2)
}
func (PersistenceRepository) CountEmailList(p0 string, p1 []any, p2 *int64) (bool, error) {
	return Database.SQL(p0, p1...).Get(p2)
}
func (PersistenceRepository) SelectMailboxStat(p0 string, p1 int, p2 *domain.EmailStatRow) (bool, error) {
	return Database.SQL(p0, p1).Get(p2)
}
func (PersistenceRepository) SelectMailboxUIDs(p0 string, p1 []any, p2 *[]*response.UserEmailUIDData) error {
	return Database.SQL(p0, p1...).Find(p2)
}
func (PersistenceRepository) SelectInboxUIDEmails(p0 string, p1 int, p2 *[]*response.UserEmailUIDData) error {
	return Database.SQL(p0, p1, 0).Find(p2)
}
func (PersistenceRepository) SelectSentUIDEmails(p0 string, p1 int, p2 *[]*response.UserEmailUIDData) error {
	return Database.SQL(p0, p1, 1).Find(p2)
}
func (PersistenceRepository) SelectDraftUIDEmails(p0 string, p1 int, p2 *[]*response.UserEmailUIDData) error {
	return Database.SQL(p0, p1, 4).Find(p2)
}
func (PersistenceRepository) SelectDeletedUIDEmails(p0 string, p1 int, p2 *[]*response.UserEmailUIDData) error {
	return Database.SQL(p0, p1, 3).Find(p2)
}
func (PersistenceRepository) SelectJunkUIDEmails(p0 string, p1 int, p2 *[]*response.UserEmailUIDData) error {
	return Database.SQL(p0, p1, 5).Find(p2)
}
func (PersistenceRepository) SelectGroupUIDs(p0 []int, p1 *[]*response.UserEmailUIDData, p2 int, p3 int) error {
	return Database.SQL(fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and group_id = ?)) a WHERE serial_number in (%s)", array.Join(p0, ","))).Find(p1, p2, p3)
}
func (PersistenceRepository) FindEmailResponsesByID(p0 []int, p1 *[]*response.EmailResponseData) error {
	return Database.Table("email").Select("*").Where(builder.Eq{"id": p0}).Find(p1)
}
func (PersistenceRepository) InsertOutgoingEmail(p0 *models.Email) (int64, error) {
	return Database.Insert(p0)
}
func (PersistenceRepository) RecordOutgoingFailure(p0 *context.Context, p1 string, p2 int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(p0, "update email set status =2 ,error=? where id = ? "), p1, p2)
}
func (PersistenceRepository) InsertFailedEmailOwner(p0 *models.UserEmail) (int64, error) {
	return Database.Insert(p0)
}
func (PersistenceRepository) RecordOutgoingSuccess(p0 *context.Context, p1 int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(p0, "update email set status =1  where id = ? "), p1)
}
func (PersistenceRepository) UpdateUserRule(p0 *context.Context, p1 string, p2 string, p3 int, p4 string, p5 int, p6 int, p7 int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(p0, "update rule set name=? ,value = ? ,action = ?,params = ?,sort = ? where id = ? and user_id = ?"), p1, p2, p3, p4, p5, p6, p7)
}
func (PersistenceRepository) InsertUserRule(p0 *context.Context, p1 string, p2 string, p3 int, p4 int, p5 string, p6 int) (sql.Result, error) {
	return Database.Exec(DatabaseWithContext(p0, "insert into rule (name,value,user_id,action,params,sort) values (?,?,?,?,?,?)"), p1, p2, p3, p4, p5, p6)
}
func (PersistenceRepository) FindRecentUserEmails(p0 int, p1 time.Time, p2 *[]models.UserEmail) error {
	return Database.Table("user_email").Where("user_id=? and create >=?", p0, p1).Find(p2)
}
func (PersistenceRepository) FindAdministrators(p0 *[]*models.User) error {
	return Database.Table(&models.User{}).Where("is_admin=1").Find(p0)
}
func (PersistenceRepository) ListSavedEmailOwners(p0 int64, p1 *[]*models.UserEmail) error {
	return Database.Table(&models.UserEmail{}).Where("email_id=?", p0).Find(p1)
}
