package // PersistencePort isolates compatibility queries from use-case orchestration.
app

import (
	"database/sql"
	"time"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
)

type PersistencePort interface {
	FindEmailByID(p0 int, p1 *models.Email) (bool, error)
	FindEmailOwnership(p0 int, p1 int, p2 *[]models.UserEmail) error
	FindUserEmail(p0 int, p1 int, p2 *models.UserEmail) (bool, error)
	FindEmailDetail(p0 int, p1 *response.EmailResponseData) (bool, error)
	MarkUserEmailRead(p0 int, p1 *models.UserEmail) (int64, error)
	SetEmailRead(p0 int, p1 int, p2 *models.UserEmail) (int64, error)
	FindUserGroupByName(p0 int, p1 string, p2 *models.Group) (bool, error)
	FindEmailsByID(p0 []int, p1 *[]models.Email) error
	FindGroupByName(p0 string, p1 int, p2 *models.Group) (bool, error)
	InsertGroup(p0 *models.Group) (int64, error)
	UpdateGroup(p0 int, p1 *models.Group) (int64, error)
	FindGroupByPath(p0 string, p1 int, p2 *models.Group) (bool, error)
	FindChildGroupIDs(p0 int, p1 int, p2 *[]domain.GroupIDRow) error
	MoveEmailsToGroup(p0 *context.Context, p1 []int, p2 int, p3 int) (sql.Result, error)
	FindChildGroups(p0 int, p1 int, p2 *[]*models.Group) error
	ListUserGroups(p0 int, p1 *[]*models.Group) error
	FindGroupsByParent(p0 int, p1 *[]*models.Group) error
	FindGroupByID(p0 int, p1 *models.Group) (bool, error)
	CountGroupEmails(p0 int, p1 *int) (bool, error)
	FindGroupLastUID(p0 int, p1 *int) (bool, error)
	CountGroupUnreadEmails(p0 int, p1 *int) (bool, error)
	FindLastInboxUID(p0 int, p1 *int) (bool, error)
	FindLastSentUID(p0 int, p1 *int) (bool, error)
	FindLastDraftUID(p0 int, p1 *int) (bool, error)
	FindLastDeletedUID(p0 int, p1 *int) (bool, error)
	FindLastJunkUID(p0 int, p1 *int) (bool, error)
	CountUnreadInboxEmails(p0 int, p1 *int) (bool, error)
	CountInboxEmails(p0 int, p1 *int) (bool, error)
	CountSentEmails(p0 int, p1 *int) (bool, error)
	CountUnreadDraftEmails(p0 int, p1 *int) (bool, error)
	CountDraftEmails(p0 int, p1 *int) (bool, error)
	CountUnreadDeletedEmails(p0 int, p1 *int) (bool, error)
	CountDeletedEmails(p0 int, p1 *int) (bool, error)
	CountUnreadJunkEmails(p0 int, p1 *int) (bool, error)
	CountJunkEmails(p0 int, p1 *int) (bool, error)
	MoveEmailsToInbox(p0 int, p1 []int) (int64, error)
	MoveEmailsToSent(p0 int, p1 []int) (int64, error)
	MoveEmailsToDraft(p0 int, p1 []int) (int64, error)
	MoveEmailsToJunk(p0 int, p1 []int) (int64, error)
	ListUserRules(p0 int, p1 *[]*models.Rule) error
	MarkRuleEmailRead(p0 int64, p1 int) (int64, error)
	MarkRuleEmailDeleted(p0 int64, p1 int) (int64, error)
	FindLocalForwardAccount(p0 string, p1 *models.User) (bool, error)
	FindLocalForwardOwnership(p0 int64, p1 int, p2 *models.UserEmail) (bool, error)
	InsertLocalForwardOwnership(p0 int, p1 int64, p2 int) (int64, error)
	SetEmailReceiveType(p0 int64) (int64, error)
	ResetEmailOwnerMailbox(p0 int64, p1 int) (int64, error)
	SetEmailSendType(p0 int64) (int64, error)
	SetOwnerDraftMailbox(p0 int64, p1 int) (int64, error)
	SetOwnerDeletedMailbox(p0 int64, p1 int) (int64, error)
	SetOwnerJunkMailbox(p0 int64, p1 int) (int64, error)
	SetOwnerCustomGroup(p0 int64, p1 int, p2 int) (int64, error)
	ListSetupAccounts(p0 *[]*models.User) error
	InsertAdministrator(p0 *models.User) (int64, error)
	InsertOutgoingEmail(p0 *models.Email) (int64, error)
	RecordOutgoingFailure(p0 *context.Context, p1 string, p2 int) (sql.Result, error)
	InsertFailedEmailOwner(p0 *models.UserEmail) (int64, error)
	RecordOutgoingSuccess(p0 *context.Context, p1 int) (sql.Result, error)
	FindLoginAccount(p0 string, p1 string, p2 *models.User) (bool, error)
	UpdateUserRule(p0 *context.Context, p1 string, p2 string, p3 int, p4 string, p5 int, p6 int, p7 int) (sql.Result, error)
	InsertUserRule(p0 *context.Context, p1 string, p2 string, p3 int, p4 int, p5 string, p6 int) (sql.Result, error)
	DeleteUserRule(p0 *context.Context, p1 int, p2 int) (sql.Result, error)
	FindTokenAccount(p0 string, p1 *models.User) (bool, error)
	SetAccountPassword(p0 int, p1 string) (int64, error)
	ListAccounts(p0 int, p1 int, p2 *[]models.User) (int64, error)
	FindAccountByID(p0 int, p1 *models.User) (bool, error)
	FindAccountByName(p0 string, p1 *models.User) (bool, error)
	UpdateAccount(p0 int, p1 *models.User) (int64, error)
	FindGroupsByPathPattern(p0 int, p1 string, p2 *[]*models.Group) error
	FindIMAPAccount(p0 string, p1 string, p2 *models.User) (bool, error)
	FindRecentUserEmails(p0 int, p1 time.Time, p2 *[]models.UserEmail) error
	FindAPOPAccount(p0 string, p1 *models.User) (bool, error)
	FindAdministrators(p0 *[]*models.User) error
	ListSavedEmailOwners(p0 int64, p1 *[]*models.UserEmail) error
}

var Persistence PersistencePort
