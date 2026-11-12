package app

import (
	"time"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
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
