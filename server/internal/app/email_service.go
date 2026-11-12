package app

import (
	"github.com/Jinnrry/pmail/consts"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/hooks/contract"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// EmailServiceDeps 邮件写用例依赖：持久化端口、删除事务、收信能力与插件钩子。
type EmailServiceDeps struct {
	Emails              EmailPersistence
	Users               UserPersistence
	Audit               EmailAuditPersistence
	OpenDeletionSession func() EmailDeletionSession
	NotifyMailbox       func(*context.Context, int, *models.Email) error
	CheckSPF            func(string, *parsemail.User, string) bool
	Hooks               func() []contract.EmailHook
	DeliveryError       func(error, map[string]error) error
	Rules               *RuleService
}

// EmailService 邮件写用例：收信、发信、删除、移动、复制与已读状态。
type EmailService struct {
	emails        EmailPersistence
	users         UserPersistence
	audit         EmailAuditPersistence
	openDeletion  func() EmailDeletionSession
	notifyMailbox func(*context.Context, int, *models.Email) error
	checkSPF      func(string, *parsemail.User, string) bool
	hooks         func() []contract.EmailHook
	deliveryError func(error, map[string]error) error
	rules         *RuleService
}

func NewEmailService(deps EmailServiceDeps) *EmailService {
	return &EmailService{
		emails:        deps.Emails,
		users:         deps.Users,
		audit:         deps.Audit,
		openDeletion:  deps.OpenDeletionSession,
		notifyMailbox: deps.NotifyMailbox,
		checkSPF:      deps.CheckSPF,
		hooks:         deps.Hooks,
		deliveryError: deps.DeliveryError,
		rules:         deps.Rules,
	}
}

// EmailSvc 邮件写服务实例，由 bootstrap.BindDependencies 装配
var EmailSvc *EmailService

func (s *EmailService) MarkRead(ctx *context.Context, emailId int, hadRead bool) {
	ue := models.UserEmail{
		UserID:  ctx.UserID,
		IsRead:  1,
		EmailID: emailId,
	}
	if !hadRead {
		ue.IsRead = 0
	}
	s.emails.SetEmailRead(emailId, ctx.UserID, &ue)
}

func (s *EmailService) MoveToGroup(ctx *context.Context, mailIDs []int, groupID int) bool {
	res, err := s.emails.MoveEmailsToGroup(ctx, mailIDs, groupID, ctx.UserID)
	if err != nil {
		log.WithContext(ctx).Errorf("SQL Error:%+v", err)
		return false
	}
	rowNum, err := res.RowsAffected()
	if err != nil {
		log.WithContext(ctx).Errorf("SQL Error:%+v", err)
		return false
	}
	return rowNum > 0
}

func (s *EmailService) MoveToDefaultBox(ctx *context.Context, mailIDs []int, groupName string) error {
	switch groupName {
	case "Deleted Messages":
		return s.Delete(ctx, mailIDs, false)
	case "INBOX":
		_, err := s.emails.MoveEmailsToInbox(ctx.UserID, mailIDs)
		return err
	case "Sent Messages":
		_, err := s.emails.MoveEmailsToSent(ctx.UserID, mailIDs)
		return err
	case "Drafts":
		_, err := s.emails.MoveEmailsToDraft(ctx.UserID, mailIDs)
		return err
	case "Junk":
		_, err := s.emails.MoveEmailsToJunk(ctx.UserID, mailIDs)
		return err
	}
	return nil
}

func (s *EmailService) CopyToDefaultBox(ctx *context.Context, mails []*response.EmailResponseData, dest string) (int, []int, error) {
	var destUIDs []int
	for _, email := range mails {
		newUE := models.UserEmail{
			UserID:  ctx.UserID,
			EmailID: email.Id,
			IsRead:  email.IsRead,
			GroupId: 0,
		}
		switch dest {
		case "Deleted Messages":
			newUE.Status = consts.EmailStatusDel
		case "INBOX":
			newUE.Status = consts.EmailStatusWait
		case "Sent Messages":
			newUE.Status = consts.EmailStatusSent
		case "Drafts":
			newUE.Status = consts.EmailStatusDrafts
		case "Junk":
			newUE.Status = consts.EmailStatusJunk
		}
		s.emails.InsertFailedEmailOwner(&newUE)
		destUIDs = append(destUIDs, newUE.ID)
	}
	return models.GroupNameToCode[dest], destUIDs, nil
}

func (s *EmailService) CopyToGroup(ctx *context.Context, mails []*response.EmailResponseData, groupID int) (int, []int, error) {
	var destUIDs []int
	for _, email := range mails {
		newUE := models.UserEmail{
			UserID:  ctx.UserID,
			EmailID: email.Id,
			IsRead:  email.IsRead,
			GroupId: groupID,
			Status:  email.Status,
		}
		s.emails.InsertFailedEmailOwner(&newUE)
		destUIDs = append(destUIDs, newUE.ID)
	}
	return groupID, destUIDs, nil
}
