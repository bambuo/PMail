package app

import (
	"github.com/Jinnrry/pmail/internal/domain"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/pkg/context"
)

func ListEmails(ctx *context.Context, tagInfo dto.SearchTag, keyword string, pop3List bool, offset, limit int) (emailList []*response.EmailResponseData, total int64) {
	return QueryEmailList(ctx, tagInfo, keyword, pop3List, offset, limit)
}

// 发件箱不展示已删除的邮件

type EmailStatRow = domain.EmailStatRow

// Stat 查询邮件总数和大小

type EmailListImapListReq struct {
	UidList []int
	Star    int
	End     int
}

var QueryEmailList func(*context.Context, dto.SearchTag, string, bool, int, int) ([]*response.EmailResponseData, int64)
var QueryMailboxStat func(*context.Context) (int64, int64)
var QueryUIDList func(*context.Context, string, int, int, []int) []*response.UserEmailUIDData
var QueryMailboxEmails func(*context.Context, string, EmailListImapListReq, bool) []*response.EmailResponseData

func MailboxStat(ctx *context.Context) (int64, int64) { return QueryMailboxStat(ctx) }
func EmailListGetUEListByUID(ctx *context.Context, name string, start, end int, ids []int) []*response.UserEmailUIDData {
	return QueryUIDList(ctx, name, start, end, ids)
}
func EmailListGetEmailListByGroup(ctx *context.Context, name string, req EmailListImapListReq, uid bool) []*response.EmailResponseData {
	return QueryMailboxEmails(ctx, name, req, uid)
}
