package app

import (
	"github.com/Jinnrry/pmail/internal/domain"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// HasAccess 检查当前用户是否有某封邮件的访问权
func (s *EmailQueryService) HasAccess(ctx *context.Context, email *models.Email) bool {
	if ctx.IsAdmin {
		return true
	}
	var ue []models.UserEmail
	err := s.emails.FindEmailOwnership(email.Id, ctx.UserID, &ue)
	if err != nil {
		log.Errorf("Error while checking user: %v", err)
		return false
	}

	return domain.CanAccessEmail(ctx.IsAdmin, len(ue))
}
