package infra

import (
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/internal/app"

	"github.com/Jinnrry/pmail/internal/domain"

	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

func (r PersistenceRepository) QueryEmailList(ctx *context.Context, tagInfo dto.SearchTag, keyword string, pop3List bool, offset, limit int) (emailList []*response.EmailResponseData, total int64) {
	querySQL, queryParams := emailListGenSQL(ctx, false, tagInfo, keyword, pop3List, offset, limit)

	err := PersistenceRepository{}.SelectEmailList(querySQL, queryParams, &emailList)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL ERROR: %s ,Error:%s", querySQL, err)
	}

	totalSQL, totalParams := emailListGenSQL(ctx, true, tagInfo, keyword, pop3List, offset, limit)

	_, err = PersistenceRepository{}.CountEmailList(totalSQL, totalParams, &total)

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
	_, err := PersistenceRepository{}.SelectMailboxStat(sql, ctx.UserID, &ret)
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
		PersistenceRepository{}.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			return nil
		}

		sql += " and group_id = ?"
		params = append(params, group.ID)
	}
	PersistenceRepository{}.SelectMailboxUIDs(sql, params, &ue)

	return ue
}

func emailListGetEmailListByUidList(ctx *context.Context, groupName string, req app.EmailListImapListReq, uid bool) []*response.EmailResponseData {
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
		err = PersistenceRepository{}.SelectInboxUIDEmails(sql, ctx.UserID, &ue)
	case "Sent Messages":
		err = PersistenceRepository{}.SelectSentUIDEmails(sql, ctx.UserID, &ue)
	case "Drafts":
		err = PersistenceRepository{}.SelectDraftUIDEmails(sql, ctx.UserID, &ue)
	case "Deleted Messages":
		err = PersistenceRepository{}.SelectDeletedUIDEmails(sql, ctx.UserID, &ue)
	case "Junk":
		err = PersistenceRepository{}.SelectJunkUIDEmails(sql, ctx.UserID, &ue)
	default:
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		PersistenceRepository{}.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			return ret
		}
		err = PersistenceRepository{}.SelectGroupUIDs(req.UidList, &ue, ctx.UserID, group.ID)
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

	_ = PersistenceRepository{}.FindEmailResponsesByID(emailIds, &ret)

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
		return emailListGetEmailListByUidList(ctx, groupName, req, uid)
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
		PersistenceRepository{}.SelectInboxUIDEmails(sql, ctx.UserID, &ue)
	case "Sent Messages":
		PersistenceRepository{}.SelectSentUIDEmails(sql, ctx.UserID, &ue)
	case "Drafts":
		PersistenceRepository{}.SelectDraftUIDEmails(sql, ctx.UserID, &ue)
	case "Deleted Messages":
		PersistenceRepository{}.SelectDeletedUIDEmails(sql, ctx.UserID, &ue)
	case "Junk":
		PersistenceRepository{}.SelectJunkUIDEmails(sql, ctx.UserID, &ue)
	default:
		groupNames := strings.Split(groupName, "/")
		groupName = groupNames[len(groupNames)-1]

		var group models.Group
		PersistenceRepository{}.FindUserGroupByName(ctx.UserID, groupName, &group)
		if group.ID == 0 {
			return ret
		}
		PersistenceRepository{}.SelectGroupUIDs(req.UidList, &ue, ctx.UserID, group.ID)
	}

	ueMap := map[int]*response.UserEmailUIDData{}
	var emailIds []int
	for _, email := range ue {
		ueMap[email.EmailID] = email
		emailIds = append(emailIds, email.EmailID)
	}

	_ = PersistenceRepository{}.FindEmailResponsesByID(emailIds, &ret)

	for i, data := range ret {
		ret[i].IsRead = ueMap[data.Id].IsRead
		ret[i].SerialNumber = ueMap[data.Id].SerialNumber
		ret[i].UeId = ueMap[data.Id].ID
	}

	return ret
}
