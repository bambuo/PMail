package api

import (
	"io"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
)

func (s *SMTPSession) Data(r io.Reader) error { return (*app.MailSubmission)(s).Receive(r) }
func (s *SMTPSession) deliverOutgoingEmail(email *parsemail.Email, sender func(*context.Context, *parsemail.Email) (error, map[string]error)) error {
	return (*app.MailSubmission)(s).DeliverOutgoingEmail(email, sender)
}

// Compatibility helpers keep existing package-level regression tests on the same paths.
func smtpSaveEmail(ctx *context.Context, size int, email *parsemail.Email, sendUserID, emailType int, recipients []string, spf, dkim bool) ([]*models.User, *models.Email, error) {
	return app.SaveEmail(ctx, size, email, sendUserID, emailType, recipients, spf, dkim)
}
