// 持久化端口按模块拆分为角色接口（User/Email/Group/MailboxStats/Rule），
// 每个消费方只依赖自己用到的角色接口；PersistencePort 为其组合，
// 由 infra.PersistenceRepository 统一实现、bootstrap.BindDependencies 装配。
package app

import (
	"database/sql"
	"time"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
)

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
	MoveEmailsToGroup(ctx *context.Context, emailIDs []int, groupID int, userID int) (sql.Result, error)
	MoveEmailsToInbox(userID int, emailIDs []int) (int64, error)
	MoveEmailsToSent(userID int, emailIDs []int) (int64, error)
	MoveEmailsToDraft(userID int, emailIDs []int) (int64, error)
	MoveEmailsToJunk(userID int, emailIDs []int) (int64, error)
	InsertOutgoingEmail(email *models.Email) (int64, error)
	RecordOutgoingFailure(ctx *context.Context, errMsg string, emailID int) (sql.Result, error)
	RecordOutgoingSuccess(ctx *context.Context, emailID int) (sql.Result, error)
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
	UpdateUserRule(ctx *context.Context, name string, value string, action int, params string, sort int, ruleID int, userID int) (sql.Result, error)
	InsertUserRule(ctx *context.Context, name string, value string, userID int, action int, params string, sort int) (sql.Result, error)
	DeleteUserRule(ctx *context.Context, ruleID int, userID int) (sql.Result, error)
}

// PersistencePort 组合端口：兼容既有消费方，能力等于五个角色接口之和
type PersistencePort interface {
	UserPersistence
	EmailPersistence
	GroupPersistence
	MailboxStatsPersistence
	RulePersistence
}

var Persistence PersistencePort
