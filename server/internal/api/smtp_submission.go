package api

import (
	"io"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
)

func (s *SMTPSession) Data(r io.Reader) error { return (*app.MailSubmission)(s).Receive(r) }
func (s *SMTPSession) deliverOutgoingEmail(email *parsemail.Email, sender func(*context.Context, *parsemail.Email) (error, map[string]error)) error {
	return (*app.MailSubmission)(s).DeliverOutgoingEmail(email, sender)
}
