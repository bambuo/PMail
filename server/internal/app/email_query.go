package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// EmailQueryService 邮件读取用例：列表、详情、搜索、附件与访问控制。
type EmailQueryService struct {
	emails  EmailPersistence
	queries EmailQueryPersistence
}

func NewEmailQueryService(emails EmailPersistence, queries EmailQueryPersistence) *EmailQueryService {
	return &EmailQueryService{emails: emails, queries: queries}
}

// EmailQuerySvc 邮件读取服务实例，由 bootstrap.BindDependencies 装配
var EmailQuerySvc *EmailQueryService

type EmailListImapListReq struct {
	UidList []int
	Star    int
	End     int
}

type EmailStatRow = domain.EmailStatRow

func (s *EmailQueryService) List(ctx *context.Context, tagInfo dto.SearchTag, keyword string, pop3List bool, offset, limit int) (emailList []*response.EmailResponseData, total int64) {
	return s.queries.QueryEmailList(ctx, tagInfo, keyword, pop3List, offset, limit)
}

func (s *EmailQueryService) MailboxStat(ctx *context.Context) (int64, int64) {
	return s.queries.MailboxStat(ctx)
}

func (s *EmailQueryService) ListByUID(ctx *context.Context, groupName string, start, end int, ids []int) []*response.UserEmailUIDData {
	return s.queries.EmailListGetUEListByUID(ctx, groupName, start, end, ids)
}

func (s *EmailQueryService) ListByGroup(ctx *context.Context, groupName string, req EmailListImapListReq, uid bool) []*response.EmailResponseData {
	return s.queries.EmailListGetEmailListByGroup(ctx, groupName, req, uid)
}

func (s *EmailQueryService) FindUserEmails(ctx *context.Context, groupName string, req EmailListImapListReq, uid bool) []models.UserEmail {
	return s.queries.EmailDetailFindUE(ctx, groupName, req, uid)
}

func (s *EmailQueryService) RecentUserEmails(ctx *context.Context, since time.Time) []models.UserEmail {
	var ues []models.UserEmail
	if err := s.emails.FindRecentUserEmails(ctx.UserID, since, &ues); err != nil {
		return nil
	}
	return ues
}

func (s *EmailQueryService) Detail(ctx *context.Context, id int, markRead bool) (*response.EmailResponseData, error) {
	// 先查是否是本人的邮件
	var ue models.UserEmail
	_, err := s.emails.FindUserEmail(id, ctx.UserID, &ue)
	if err != nil {
		log.Error(err)
	}
	if ue.ID == 0 && !ctx.IsAdmin {
		return nil, errors.New("Not authorized")
	}

	//获取邮件内容
	var email response.EmailResponseData
	_, err = s.emails.FindEmailDetail(id, &email)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL error:%+v", err)
		return nil, err
	}

	email.IsRead = ue.IsRead

	if markRead && ue.IsRead == 0 {
		ue.IsRead = 1
		_, err = s.emails.MarkUserEmailRead(ue.ID, &ue)
		if err != nil {
			log.WithContext(ctx).Errorf("SQL error:%+v", err)
		}
	}

	// 将内容中的cid内容替换成url
	if email.Attachments != "" {
		var atts []parsemail.Attachment
		_ = json.Unmarshal([]byte(email.Attachments), &atts)
		for _, att := range atts {
			email.Html = sql.NullString{
				String: strings.ReplaceAll(email.Html.String, fmt.Sprintf("cid:%s", att.ContentID), fmt.Sprintf("/attachments/%d/%s", id, att.ContentID)),
			}
		}
	}

	return &email, nil
}

func (s *EmailQueryService) Attachments(ctx *context.Context, emailId int, cid string) (string, []byte) {
	var email models.Email
	_, err := s.emails.FindEmailByID(emailId, &email)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL error:%+v", err)
		return "", nil
	}

	if !s.HasAccess(ctx, &email) {
		return "", nil
	}

	var atts []parsemail.Attachment
	_ = json.Unmarshal([]byte(email.Attachments), &atts)
	for _, att := range atts {
		if att.ContentID == cid {
			return att.ContentType, att.Content
		}
	}
	return "", nil
}

func (s *EmailQueryService) AttachmentsByIndex(ctx *context.Context, emailId int, index int) (string, []byte) {
	var email models.Email
	_, err := s.emails.FindEmailByID(emailId, &email)

	if err != nil {
		log.WithContext(ctx).Errorf("SQL error:%+v", err)
		return "", nil
	}

	if !s.HasAccess(ctx, &email) {
		return "", nil
	}

	var atts []parsemail.Attachment
	_ = json.Unmarshal([]byte(email.Attachments), &atts)

	if len(atts) > index {
		return atts[index].Filename, atts[index].Content
	}
	return "", nil
}
