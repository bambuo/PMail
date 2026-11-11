package app

import (
	"fmt"
	"strings"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/send"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

func RuleGetAllRules(ctx *context.Context, userId int) []*dto.Rule {
	var res []*models.Rule
	var err error
	if userId == 0 {
		return nil
	} else {
		err = Persistence.ListUserRules(userId, &res)

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

func RuleMatchRule(ctx *context.Context, rule *dto.Rule, email *parsemail.Email) bool {

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

func RuleDoRule(ctx *context.Context, rule *dto.Rule, email *parsemail.Email, user *models.User, rawEmailData []byte) {
	log.WithContext(ctx).Debugf("执行规则:%s", rule.Name)

	switch rule.Action {
	case dto.READ:
		if email.MessageId > 0 {
			_, err := Persistence.MarkRuleEmailRead(email.MessageId, rule.UserId)
			if err != nil {
				log.WithContext(ctx).Errorf("sqlERror :%v", err)
			}
		}
	case dto.DELETE:
		_, err := Persistence.MarkRuleEmailDeleted(email.MessageId, rule.UserId)
		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	case dto.FORWARD:
		err := ForwardEmail(ctx, email, rule.Params, user, rawEmailData)
		if err != nil {
			log.WithContext(ctx).Errorf("Forward Error:%v", err)
		} else {
			log.WithContext(ctx).Infof("Forward Success:%s@%s -> %s", user.Account, config.Instance.Domains[0], rule.Params)
		}
	case dto.MOVE:
		ruleDoMove(ctx, rule, email, user)
	}

}

func ForwardEmail(ctx *context.Context, email *parsemail.Email, forwardAddress string, user *models.User, rawEmailData []byte) error {
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
		return ruleForwardToLocalUser(ctx, email, account, forwardAddress)
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

func ruleForwardToLocalUser(ctx *context.Context, email *parsemail.Email, account, forwardAddress string) error {
	if email.MessageId <= 0 {
		return fmt.Errorf("email has not been saved before local forwarding")
	}

	var user models.User
	has, err := Persistence.FindLocalForwardAccount(account, &user)

	if err != nil {
		return err
	}
	if !has {
		return fmt.Errorf("local forward user not found: %s", forwardAddress)
	}

	var ue models.UserEmail
	has, err = Persistence.FindLocalForwardOwnership(email.MessageId, user.ID, &ue)

	if err != nil {
		return err
	}
	if has {
		return nil
	}

	_, err = Persistence.InsertLocalForwardOwnership(user.ID, email.MessageId, email.Status)

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

func ruleDoMove(ctx *context.Context, rule *dto.Rule, email *parsemail.Email, user *models.User) {

	groupId := cast.ToInt(rule.Params)
	switch groupId {
	case models.INBOX:
		_, err := Persistence.SetEmailReceiveType(email.MessageId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

		_, err = Persistence.ResetEmailOwnerMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	case models.Sent:
		_, err := Persistence.SetEmailSendType(email.MessageId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

		_, err = Persistence.ResetEmailOwnerMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

	case models.Drafts:
		_, err := Persistence.SetOwnerDraftMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

	case models.Deleted:
		_, err := Persistence.SetOwnerDeletedMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	case models.Junk:
		_, err := Persistence.SetOwnerJunkMailbox(email.MessageId, rule.UserId)

		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}

	default:

		_, err := Persistence.SetOwnerCustomGroup(email.MessageId, rule.UserId, groupId)
		if err != nil {
			log.WithContext(ctx).Errorf("sqlERror :%v", err)
		}
	}

}
