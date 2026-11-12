package persistence

import (
	"database/sql"
	stderrors "errors"
	"fmt"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
	"xorm.io/builder"
	"xorm.io/xorm"
)

func (r PersistenceRepository) QueryEmailList(ctx *context.Context, tagInfo dto.SearchTag, keyword string, pop3List bool, offset, limit int) (emailList []*response.EmailResponseData, total int64) {
	querySQL, queryParams := emailListGenSQL(ctx, false, tagInfo, keyword, pop3List, offset, limit)

	err := r.SelectEmailList(querySQL, queryParams, &emailList)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL ERROR: %s ,Error:%s", querySQL, err)
	}

	totalSQL, totalParams := emailListGenSQL(ctx, true, tagInfo, keyword, pop3List, offset, limit)

	_, err = r.CountEmailList(totalSQL, totalParams, &total)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL ERROR: %s ,Error:%s", querySQL, err)
	}

	return emailList, total
}

func emailListGenSQL(ctx *context.Context, count bool, tagInfo dto.SearchTag, keyword string, pop3List bool, offset, limit int) (string, []any) {
	sqlParams := []any{ctx.UserID}
	sql := "select "

	if count {
		sql += `count(1) from email e left join user_email ue on e.id=ue.email_id where ue.user_id = ? `
	} else if pop3List {
		sql += `e.id,e.size from email e left join user_email ue on e.id=ue.email_id where ue.user_id = ? `
	} else {
		sql += `e.*,ue.is_read from email e left join user_email ue on e.id=ue.email_id where ue.user_id = ? `
	}

	if tagInfo.Status != -1 {
		switch tagInfo.Status {
		case consts.EmailStatusDel:
			sql += " and (ue.status =? or ue.group_id=?) "
			sqlParams = append(sqlParams, tagInfo.Status, models.Deleted)
		case consts.EmailStatusDrafts:
			sql += " and (ue.status =? or ue.group_id=?) "
			sqlParams = append(sqlParams, tagInfo.Status, models.Drafts)
		case consts.EmailStatusJunk:
			sql += " and (ue.status =? or ue.group_id=?) "
			sqlParams = append(sqlParams, tagInfo.Status, models.Junk)
		}

	} else if tagInfo.Status == -1 {
		if tagInfo.Type != 1 {
			sql += " and ue.status = 0"
		} else {

			sql += " and ue.status != 3"
		}
	}

	if tagInfo.Type == consts.EmailTypeReceive {
		sql += " and type =? "
		sqlParams = append(sqlParams, tagInfo.Type)
	} else if tagInfo.Type == consts.EmailTypeSend {
		sql += " and (type =? or ue.group_id=?)"
		sqlParams = append(sqlParams, tagInfo.Type, models.Sent)
	}

	if tagInfo.GroupId != -1 {
		if tagInfo.GroupId > 0 {
			sql += " and ue.group_id=? "
			sqlParams = append(sqlParams, tagInfo.GroupId)
		} else if tagInfo.GroupId == 0 && tagInfo.Status == -1 {
			sql += " and (ue.group_id=? or ue.group_id=0)"
			sqlParams = append(sqlParams, models.INBOX)
		}
	} else {
		sql += " and (ue.group_id=0 or ue.group_id =?) "
		sqlParams = append(sqlParams, models.INBOX)
	}

	if keyword != "" {
		sql += " and (subject like ? or text like ? )"
		sqlParams = append(sqlParams, "%"+keyword+"%", "%"+keyword+"%")
	}

	if limit == 0 {
		limit = 10
	}

	sql += " order by e.id desc"

	if limit < 10000 {
		sql += fmt.Sprintf(" LIMIT %d OFFSET %d ", limit, offset)
	}

	return sql, sqlParams

}

// Stat 查询邮件总数和大小
func (r PersistenceRepository) MailboxStat(ctx *context.Context) (int64, int64) {
	sql := `select count(1) as total,sum(size) as size from email e left join user_email ue on e.id=ue.email_id where ue.user_id = ? and e.type = 0 and ue.status != 3`
	var ret domain.EmailStatRow
	_, err := r.SelectMailboxStat(sql, ctx.UserID, &ret)
	if err != nil {
		log.WithContext(ctx).Errorf("SQL ERROR: %s ,Error:%s", sql, err)
	}
	return ret.Total, ret.Size
}

func (r PersistenceRepository) EmailListGetUEListByUID(ctx *context.Context, groupName string, star, end int, uidList []int) []*response.UserEmailUIDData {
	var ue []*response.UserEmailUIDData
	sql := "SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE user_id = ? "

	params := []any{ctx.UserID}

	if len(uidList) > 0 {
		sql += fmt.Sprintf(" and id in (%s)", array.Join(uidList, ","))
	}
	if star > 0 {
		sql += " and id >=?"
		params = append(params, star)
	}
	if end > 0 {
		sql += " and id <=?"
		params = append(params, end)
	}

	switch groupName {
	case "INBOX":
		sql += " and status =?"
		params = append(params, 0)
	case "Sent Messages":
		sql += " and status =?"
		params = append(params, 1)
	case "Drafts":
		sql += " and status =?"
		params = append(params, 4)
	case "Deleted Messages":
		sql += " and status =?"
		params = append(params, 3)
	case "Junk":
		sql += " and status =?"
		params = append(params, 5)
	default:
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		r.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			return nil
		}

		sql += " and group_id = ?"
		params = append(params, group.ID)
	}
	r.SelectMailboxUIDs(sql, params, &ue)

	return ue
}

func (r *PersistenceRepository) emailListGetEmailListByUidList(ctx *context.Context, groupName string, req app.EmailListImapListReq, uid bool) []*response.EmailResponseData {
	var ret []*response.EmailResponseData
	var ue []*response.UserEmailUIDData
	sql := fmt.Sprintf("SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and id in (%s) and status = ?)", array.Join(req.UidList, ","))
	if req.Star > 0 && req.End != 0 {
		sql = fmt.Sprintf("SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and id >=%d and id <= %d and status = ?)", req.Star, req.End)
	}
	if req.Star > 0 && req.End == 0 {
		sql = fmt.Sprintf("SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and id >=%d and status = ?)", req.Star)
	}

	var err error
	switch groupName {
	case "INBOX":
		err = r.SelectInboxUIDEmails(sql, ctx.UserID, &ue)
	case "Sent Messages":
		err = r.SelectSentUIDEmails(sql, ctx.UserID, &ue)
	case "Drafts":
		err = r.SelectDraftUIDEmails(sql, ctx.UserID, &ue)
	case "Deleted Messages":
		err = r.SelectDeletedUIDEmails(sql, ctx.UserID, &ue)
	case "Junk":
		err = r.SelectJunkUIDEmails(sql, ctx.UserID, &ue)
	default:
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		r.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			return ret
		}
		err = r.SelectGroupUIDs(req.UidList, &ue, ctx.UserID, group.ID)
	}

	if err != nil {
		log.WithContext(ctx).Errorf("SQL ERROR: %s ,Error:%s", sql, err)
	}
	ueMap := map[int]*response.UserEmailUIDData{}
	var emailIds []int
	for _, email := range ue {
		ueMap[email.EmailID] = email
		emailIds = append(emailIds, email.EmailID)
	}

	_ = r.FindEmailResponsesByID(emailIds, &ret)

	for i, data := range ret {
		ret[i].IsRead = ueMap[data.Id].IsRead
		ret[i].SerialNumber = ueMap[data.Id].SerialNumber
		ret[i].UeId = ueMap[data.Id].ID
	}

	return ret
}

func (r PersistenceRepository) EmailListGetEmailListByGroup(ctx *context.Context, groupName string, req app.EmailListImapListReq, uid bool) []*response.EmailResponseData {
	if len(req.UidList) == 0 && req.Star == 0 && req.End == 0 {
		return nil
	}

	if uid {
		return r.emailListGetEmailListByUidList(ctx, groupName, req, uid)
	}

	var ret []*response.EmailResponseData
	var ue []*response.UserEmailUIDData

	sql := fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and status = ? and group_id=0 )) a WHERE serial_number in (%s)", array.Join(req.UidList, ","))
	if req.Star > 0 && req.End == 0 {
		sql = fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and status = ? and group_id=0 )) a WHERE serial_number >= %d", req.Star)
	}
	if req.Star > 0 && req.End > 0 {
		sql = fmt.Sprintf("SELECT * from (SELECT id,email_id, is_read, ROW_NUMBER() OVER (ORDER BY id) AS serial_number FROM `user_email` WHERE (user_id = ? and status = ? and group_id=0 )) a WHERE serial_number >= %d and serial_number <=%d", req.Star, req.End)
	}

	switch groupName {
	case "INBOX":
		r.SelectInboxUIDEmails(sql, ctx.UserID, &ue)
	case "Sent Messages":
		r.SelectSentUIDEmails(sql, ctx.UserID, &ue)
	case "Drafts":
		r.SelectDraftUIDEmails(sql, ctx.UserID, &ue)
	case "Deleted Messages":
		r.SelectDeletedUIDEmails(sql, ctx.UserID, &ue)
	case "Junk":
		r.SelectJunkUIDEmails(sql, ctx.UserID, &ue)
	default:
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		r.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			return ret
		}
		r.SelectGroupUIDs(req.UidList, &ue, ctx.UserID, group.ID)
	}

	ueMap := map[int]*response.UserEmailUIDData{}
	var emailIds []int
	for _, email := range ue {
		ueMap[email.EmailID] = email
		emailIds = append(emailIds, email.EmailID)
	}

	_ = r.FindEmailResponsesByID(emailIds, &ret)

	for i, data := range ret {
		ret[i].IsRead = ueMap[data.Id].IsRead
		ret[i].SerialNumber = ueMap[data.Id].SerialNumber
		ret[i].UeId = ueMap[data.Id].ID
	}

	return ret
}

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

type EmailDeletionSession struct{ *xorm.Session }

func (r *PersistenceRepository) NewDeletionSession() app.EmailDeletionSession {
	return &EmailDeletionSession{r.db().NewSession()}
}
func (s *EmailDeletionSession) MarkDeleted(userID int, emailID int64) error {
	_, err := s.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Update(map[string]any{"status": consts.EmailStatusDel, "group_id": 0})
	return err
}
func (s *EmailDeletionSession) FindEmailByUID(userID, uid int) (int, bool, error) {
	var relation models.UserEmail
	found, err := s.Table("user_email").Where("id=? and user_id=?", uid, userID).Get(&relation)
	return relation.EmailID, found && relation.ID != 0, err
}
func (s *EmailDeletionSession) RemoveEmailOwner(userID int, emailID int64) error {
	_, err := s.Table(&models.UserEmail{}).Where("email_id=? and user_id=?", emailID, userID).Delete(&models.UserEmail{})
	return err
}
func (s *EmailDeletionSession) RemoveEmailUID(userID, uid int) error {
	_, err := s.Table(&models.UserEmail{}).Where("id=? and user_id=?", uid, userID).Delete(&models.UserEmail{})
	return err
}
func (s *EmailDeletionSession) CountEmailOwners(emailID int64) (int, error) {
	var row struct {
		Num int `xorm:"num"`
	}
	_, err := s.Table(&models.UserEmail{}).Select("count(1) as num").Where("email_id=?", emailID).Get(&row)
	return row.Num, err
}
func (s *EmailDeletionSession) RemoveEmail(emailID int64) error {
	_, err := s.Table(&models.Email{}).Where("id=?", emailID).Delete(&models.Email{})
	return err
}

func (r PersistenceRepository) PersistDeliveryResult(ctx *context.Context, emailID int, status int8, errorText any) error {
	return WithDatabaseTransaction(r.db(), func(session *xorm.Session) error {
		result, err := session.Exec(DatabaseWithContext(ctx, `update email
			set status=?, error=?, update_time=CURRENT_TIMESTAMP
			where id=?`), status, errorText, emailID)
		if err != nil {
			return err
		}
		if err = smtpRequireOneAffected(result, "update email delivery result"); err != nil {
			return err
		}

		result, err = session.Exec(DatabaseWithContext(ctx, `update user_email
			set status=? where email_id=?`), status, emailID)
		if err != nil {
			return err
		}
		return smtpRequireOneAffected(result, "update user_email delivery result")
	})
}

func smtpRequireOneAffected(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: read affected rows: %w", operation, err)
	}
	if affected != 1 {
		return fmt.Errorf("%s: affected %d rows, want 1", operation, affected)
	}
	return nil
}

func (r PersistenceRepository) PersistInitialAudit(ctx *context.Context, modelEmail models.Email, userIDs []int) (*models.Email, error) {
	var saved models.Email
	err := WithDatabaseTransaction(r.db(), func(session *xorm.Session) error {
		candidate := modelEmail
		candidate.Id = 0
		if _, err := session.Insert(&candidate); err != nil {
			return err
		}
		if candidate.Id <= 0 {
			return stderrors.New("email insert returned no id")
		}
		for _, userID := range userIDs {
			relation := models.UserEmail{EmailID: candidate.Id, UserID: userID, Status: candidate.Status}
			if _, err := session.Insert(&relation); err != nil {
				return err
			}
		}
		saved = candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (r PersistenceRepository) FindRecipientAccounts(accounts []string, users *[]*models.User) error {
	where, params, err := builder.ToSQL(builder.In("LOWER(account)", accounts))
	if err != nil {
		return err
	}
	return r.db().Table(&models.User{}).Where(where, params...).Find(users)
}
