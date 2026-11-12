package app

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
)

// 持久化端口按模块拆分为角色接口（User/Email/Group/MailboxStats/Rule），
// 每个消费方只依赖自己用到的角色接口；由 persistence.PersistenceRepository
// 统一实现、bootstrap.BindDependencies 装配到各领域服务。

// UserPersistence 用户与账号模块持久化端口
type UserPersistence interface {
	FindTokenAccount(account string, user *models.User) (bool, error)
	FindAPOPAccount(account string, user *models.User) (bool, error)
	SetAccountPassword(userID int, hash string) (int64, error)
	ListAccounts(limit int, offset int, users *[]models.User) (int64, error)
	FindAccountByID(id int, user *models.User) (bool, error)
	FindAccountByName(account string, user *models.User) (bool, error)
	UpdateAccount(id int, user *models.User) (int64, error)
	ListSetupAccounts(users *[]*models.User) error
	InsertAdministrator(user *models.User) (int64, error)
	FindAdministrators(admins *[]*models.User) error
}

// EmailPersistence 邮件内容、归属与收发状态模块持久化端口
type EmailPersistence interface {
	FindEmailByID(id int, email *models.Email) (bool, error)
	FindEmailsByID(ids []int, emails *[]models.Email) error
	FindEmailDetail(id int, detail *response.EmailResponseData) (bool, error)
	FindEmailOwnership(emailID int, userID int, owners *[]models.UserEmail) error
	FindUserEmail(emailID int, userID int, ue *models.UserEmail) (bool, error)
	MarkUserEmailRead(ueID int, ue *models.UserEmail) (int64, error)
	SetEmailRead(emailID int, userID int, ue *models.UserEmail) (int64, error)
	MoveEmailsToGroup(ctx *pcontext.Context, emailIDs []int, groupID int, userID int) (sql.Result, error)
	MoveEmailsToInbox(userID int, emailIDs []int) (int64, error)
	MoveEmailsToSent(userID int, emailIDs []int) (int64, error)
	MoveEmailsToDraft(userID int, emailIDs []int) (int64, error)
	MoveEmailsToJunk(userID int, emailIDs []int) (int64, error)
	InsertOutgoingEmail(email *models.Email) (int64, error)
	RecordOutgoingFailure(ctx *pcontext.Context, errMsg string, emailID int) (sql.Result, error)
	RecordOutgoingSuccess(ctx *pcontext.Context, emailID int) (sql.Result, error)
	InsertFailedEmailOwner(ue *models.UserEmail) (int64, error)
	FindRecentUserEmails(userID int, since time.Time, ues *[]models.UserEmail) error
	ListSavedEmailOwners(emailID int64, owners *[]*models.UserEmail) error
	SetEmailReceiveType(emailID int64) (int64, error)
	ResetEmailOwnerMailbox(emailID int64, userID int) (int64, error)
	SetEmailSendType(emailID int64) (int64, error)
	SetOwnerDraftMailbox(emailID int64, userID int) (int64, error)
	SetOwnerDeletedMailbox(emailID int64, userID int) (int64, error)
	SetOwnerJunkMailbox(emailID int64, userID int) (int64, error)
	SetOwnerCustomGroup(emailID int64, userID int, groupID int) (int64, error)
	MarkRuleEmailRead(emailID int64, userID int) (int64, error)
	MarkRuleEmailDeleted(emailID int64, userID int) (int64, error)
	FindLocalForwardAccount(account string, user *models.User) (bool, error)
	FindLocalForwardOwnership(emailID int64, userID int, ue *models.UserEmail) (bool, error)
	InsertLocalForwardOwnership(userID int, emailID int64, status int) (int64, error)
}

// GroupPersistence 邮件分组模块持久化端口
type GroupPersistence interface {
	FindUserGroupByName(userID int, name string, group *models.Group) (bool, error)
	FindGroupByName(name string, userID int, group *models.Group) (bool, error)
	InsertGroup(group *models.Group) (int64, error)
	UpdateGroup(id int, group *models.Group) (int64, error)
	FindGroupByPath(fullPath string, userID int, group *models.Group) (bool, error)
	FindChildGroupIDs(parentID int, userID int, rows *[]domain.GroupIDRow) error
	FindChildGroups(parentID int, userID int, groups *[]*models.Group) error
	ListUserGroups(userID int, groups *[]*models.Group) error
	FindGroupsByParent(parentID int, groups *[]*models.Group) error
	FindGroupByID(id int, group *models.Group) (bool, error)
	FindGroupsByPathPattern(userID int, pattern string, groups *[]*models.Group) error
	RemoveGroups(ctx *pcontext.Context, groupIDs []int) (bool, error)
}

// EmailQueryPersistence 邮件列表、详情与搜索查询端口
type EmailQueryPersistence interface {
	QueryEmailList(ctx *pcontext.Context, tagInfo dto.SearchTag, keyword string, pop3List bool, offset, limit int) ([]*response.EmailResponseData, int64)
	MailboxStat(ctx *pcontext.Context) (int64, int64)
	EmailListGetUEListByUID(ctx *pcontext.Context, groupName string, start, end int, uidList []int) []*response.UserEmailUIDData
	EmailListGetEmailListByGroup(ctx *pcontext.Context, groupName string, req EmailListImapListReq, uid bool) []*response.EmailResponseData
	EmailDetailFindUE(ctx *pcontext.Context, groupName string, req EmailListImapListReq, uid bool) []models.UserEmail
}

// EmailAuditPersistence 邮件入库审计与投递结果持久化端口
type EmailAuditPersistence interface {
	PersistInitialAudit(ctx *pcontext.Context, email models.Email, userIDs []int) (*models.Email, error)
	PersistDeliveryResult(ctx *pcontext.Context, emailID int, status int8, errorText any) error
	FindRecipientAccounts(accounts []string, users *[]*models.User) error
}

// MailboxStatsPersistence 邮箱统计模块持久化端口（IMAP/POP3 状态与列表计数）
type MailboxStatsPersistence interface {
	CountGroupEmails(groupID int, count *int) (bool, error)
	FindGroupLastUID(groupID int, uid *int) (bool, error)
	CountGroupUnreadEmails(groupID int, count *int) (bool, error)
	FindLastInboxUID(userID int, uid *int) (bool, error)
	FindLastSentUID(userID int, uid *int) (bool, error)
	FindLastDraftUID(userID int, uid *int) (bool, error)
	FindLastDeletedUID(userID int, uid *int) (bool, error)
	FindLastJunkUID(userID int, uid *int) (bool, error)
	CountUnreadInboxEmails(userID int, count *int) (bool, error)
	CountInboxEmails(userID int, count *int) (bool, error)
	CountSentEmails(userID int, count *int) (bool, error)
	CountUnreadDraftEmails(userID int, count *int) (bool, error)
	CountDraftEmails(userID int, count *int) (bool, error)
	CountUnreadDeletedEmails(userID int, count *int) (bool, error)
	CountDeletedEmails(userID int, count *int) (bool, error)
	CountUnreadJunkEmails(userID int, count *int) (bool, error)
	CountJunkEmails(userID int, count *int) (bool, error)
}

// RulePersistence 收信规则模块持久化端口
type RulePersistence interface {
	ListUserRules(userID int, rules *[]*models.Rule) error
	UpdateUserRule(ctx *pcontext.Context, name string, value string, action int, params string, sort int, ruleID int, userID int) (sql.Result, error)
	InsertUserRule(ctx *pcontext.Context, name string, value string, userID int, action int, params string, sort int) (sql.Result, error)
	DeleteUserRule(ctx *pcontext.Context, ruleID int, userID int) (sql.Result, error)
}

type SessionPort interface {
	Put(context.Context, string, any)
	Get(context.Context, string) any
	Clear(context.Context) error
	LoadAndSave(http.Handler) http.Handler
}

var Sessions SessionPort

// Certificate operations are bound by bootstrap, keeping certificate libraries outside use cases.
var GetCertificateType func() string
var SetCertificate func(string, string, string) error
var GenerateCertificate func(bool) error
var HTTPChallenge func(string) (string, bool)

type DNSChallengeRecord struct {
	Type  string `json:"type"`
	Host  string `json:"host"`
	Value string `json:"value"`
	TTL   int    `json:"ttl"`
	Tips  string `json:"tips"`
}

var GetDNSChallengeRecords func(*pcontext.Context) []*DNSChallengeRecord

// UseCaseError preserves the existing application error envelope at the API boundary.
type UseCaseError struct {
	Code    int
	Message string
	Data    any
}

func (e *UseCaseError) Error() string { return e.Message }
func NewUseCaseError(code int, message string, data any) error {
	return &UseCaseError{Code: code, Message: message, Data: data}
}

// InitChan 控制初始化流程结束
var LifecycleInitChan = make(chan bool)

// RestartChan 控制程序重启
var LifecycleRestartChan = make(chan bool)

// StopChan 控制程序结束
var LifecycleStopChan = make(chan bool)
