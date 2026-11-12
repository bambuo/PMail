package app

import (
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/send"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

// RuleService 收信规则用例：查询、匹配、动作执行与转发。
type RuleService struct {
	rules  RulePersistence
	emails EmailPersistence
}

func NewRuleService(rules RulePersistence, emails EmailPersistence) *RuleService {
	return &RuleService{rules: rules, emails: emails}
}

// RuleSvc 收信规则服务实例，由 bootstrap.BindDependencies 装配
var RuleSvc *RuleService

func (s *RuleService) GetAll(ctx *context.Context, userId int) []*dto.Rule {
	var res []*models.Rule
	var err error
	if userId == 0 {
		return nil
	} else {
		err = s.rules.ListUserRules(userId, &res)

	}

	if err != nil {
		log.WithContext(ctx).Errorf("sqlERror :%v", err)
	}
	var ret []*dto.Rule
	for _, rule := range res {
		ret = append(ret, (&dto.Rule{}).Decode(rule))
	}

	return ret
}

func (s *RuleService) Match(ctx *context.Context, rule *dto.Rule, email *parsemail.Email) bool {

	for _, r := range rule.Rules {
		var m domain.RuleMatch

		switch r.Type {
		case domain.RuleTypeRegex:
			m = domain.NewRegexRule(r.Field, r.Rule)
		case domain.RuleTypeContains:
			m = domain.NewContainsRule(r.Field, r.Rule)
		case domain.RuleTypeEq:
			m = domain.NewEqualRule(r.Field, r.Rule)
		}
		if m == nil {
			continue
		}

		matched, err := m.Match(ruleGetFieldContent(r.Field, email))
		if err != nil {
			log.WithContext(ctx).Errorf("rule regex error %v", err)
		}
		if !matched {
			return false
		}
	}

	return true
}

func (s *RuleService) Apply(ctx *context.Context, rule *dto.Rule, email *parsemail.Email, user *models.User, rawEmailData []byte) {
	log.WithContext(ctx).Debugf("执行规则:%s", rule.Name)

	switch rule.Action {
	case dto.READ:
		if email.MessageId > 0 {
			_, err := s.emails.MarkRuleEmailRead(email.MessageId, rule.UserId)
			if err != nil {
				log.WithContext(ctx).Errorf("sqlERror :%v", err)
			}
		}
	case dto.DELETE:
		_, err := s.emails.MarkRuleEmailDeleted(email.MessageId, rule.UserId)
		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	case dto.FORWARD:
		err := s.Forward(ctx, email, rule.Params, user, rawEmailData)
		if err != nil {
			log.WithContext(ctx).Errorf("Forward Error:%v", err)
		} else {
			log.WithContext(ctx).Infof("Forward Success:%s@%s -> %s", user.Account, config.Instance.Domains[0], rule.Params)
		}
	case dto.MOVE:
		s.doMove(ctx, rule, email, user)
	}

}

func (s *RuleService) Forward(ctx *context.Context, email *parsemail.Email, forwardAddress string, user *models.User, rawEmailData []byte) error {
	forwardUser := parsemail.BuilderUser(forwardAddress)
	if forwardUser == nil || forwardUser.EmailAddress == "" {
		return fmt.Errorf("invalid forward address: %s", forwardAddress)
	}

	account, forwardDomain := forwardUser.GetDomainAccount()
	if account == "" || forwardDomain == "" {
		return fmt.Errorf("invalid forward address: %s", forwardAddress)
	}

	if ruleIsLocalDomain(forwardDomain) {
		if domain.IsSelfForward(account, user.Account) {
			return fmt.Errorf("loop forwarding to self: %s", forwardAddress)
		}
		return s.forwardToLocalUser(ctx, email, account, forwardAddress)
	}

	if len(rawEmailData) > 0 {
		err := send.ForwardRaw(ctx, email, rawEmailData, forwardAddress, user)
		if err == nil {
			return nil
		}
		log.WithContext(ctx).Warnf("Raw forward failed, retry remail forward:%v", err)
	}
	return send.Forward(ctx, email, forwardAddress, user)
}

func (s *RuleService) forwardToLocalUser(ctx *context.Context, email *parsemail.Email, account, forwardAddress string) error {
	if email.MessageId <= 0 {
		return fmt.Errorf("email has not been saved before local forwarding")
	}

	var user models.User
	has, err := s.emails.FindLocalForwardAccount(account, &user)

	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("local forward user not found: %s", forwardAddress)
	}

	var ue models.UserEmail
	has, err = s.emails.FindLocalForwardOwnership(email.MessageId, user.ID, &ue)

	if err != nil {
		return err
	}
	if has {
		return nil
	}

	_, err = s.emails.InsertLocalForwardOwnership(user.ID, email.MessageId, email.Status)

	return err
}

func ruleIsLocalDomain(domain string) bool {
	for _, localDomain := range config.Instance.Domains {
		if strings.EqualFold(domain, localDomain) {
			return true
		}
	}
	return strings.EqualFold(domain, config.Instance.Domain)
}

func (s *RuleService) doMove(ctx *context.Context, rule *dto.Rule, email *parsemail.Email, user *models.User) {

	groupId := cast.ToInt(rule.Params)
	switch groupId {
	case models.INBOX:
		_, err := s.emails.SetEmailReceiveType(email.MessageId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

		_, err = s.emails.ResetEmailOwnerMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	case models.Sent:
		_, err := s.emails.SetEmailSendType(email.MessageId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

		_, err = s.emails.ResetEmailOwnerMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

	case models.Drafts:
		_, err := s.emails.SetOwnerDraftMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

	case models.Deleted:
		_, err := s.emails.SetOwnerDeletedMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	case models.Junk:
		_, err := s.emails.SetOwnerJunkMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

	default:

		_, err := s.emails.SetOwnerCustomGroup(email.MessageId, rule.UserId, groupId)
		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	}

}

func (s *RuleService) Upsert(ctx *context.Context, userID int, rule *models.Rule) error {
	if rule.Id > 0 {
		_, err := s.rules.UpdateUserRule(ctx, rule.Name, rule.Value, rule.Action, rule.Params, rule.Sort, rule.Id, userID)
		return err
	}
	_, err := s.rules.InsertUserRule(ctx, rule.Name, rule.Value, userID, rule.Action, rule.Params, rule.Sort)
	return err
}

func (s *RuleService) Delete(ctx *context.Context, ruleID, userID int) error {
	_, err := s.rules.DeleteUserRule(ctx, ruleID, userID)
	return err
}

func ruleBuildUsers(users []*parsemail.User) string {
	ret := ""
	for i, u := range users {
		if i != 0 {
			ret += ","
		}
		ret += u.EmailAddress
	}
	return ret
}

func ruleGetFieldContent(field string, email *parsemail.Email) string {
	switch field {
	case "ReplyTo":
		return ruleBuildUsers(email.ReplyTo)
	case "From":
		return email.From.EmailAddress
	case "Subject":
		return email.Subject
	case "To":
		return ruleBuildUsers(email.To)
	case "Bcc":
		return ruleBuildUsers(email.Bcc)
	case "Cc":
		return ruleBuildUsers(email.Cc)
	case "Text":
		return string(email.Text)
	case "Html":
		return string(email.HTML)
	case "Sender":
		return email.Sender.EmailAddress
	case "Content":
		b := string(email.HTML)
		b2 := string(email.Text)
		return b + b2
	}
	return ""
}
