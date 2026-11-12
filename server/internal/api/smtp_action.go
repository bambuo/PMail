package api

import (
	"errors"
	"strings"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/id"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	log "github.com/sirupsen/logrus"
)

// The Backend implements SMTP server methods.
type SMTPBackend struct{}

func (bkd *SMTPBackend) NewSession(conn *smtp.Conn) (smtp.Session, error) {

	remoteAddress := conn.Conn().RemoteAddr()
	ctx := &context.Context{}
	ctx.SetValue(context.LogID, id.GenLogID())
	log.WithContext(ctx).Debugf("新SMTP连接")

	return &SMTPSession{
		RemoteAddress: remoteAddress,
		Ctx:           ctx,
		Service:       app.EmailSvc,
	}, nil
}

// A Session is returned after EHLO.
type SMTPSession app.MailSubmission

// AuthMechanisms returns a slice of available auth mechanisms
// supported in this example.
func (s *SMTPSession) AuthMechanisms() []string {
	return []string{sasl.Plain, sasl.Login}
}

// Auth is the handler for supported authenticators.
func (s *SMTPSession) Auth(mech string) (sasl.Server, error) {
	log.WithContext(s.Ctx).Debugf("Auth :%s", mech)
	if mech == sasl.Plain {
		return sasl.NewPlainServer(func(identity, username, password string) error {
			return s.AuthPlain(username, password)
		}), nil
	}

	if mech == sasl.Login {
		return SMTPNewLoginServer(func(username, password string) error {
			return s.AuthPlain(username, password)
		}), nil
	}

	return nil, errors.New("Auth Not Supported")
}

func (s *SMTPSession) AuthPlain(username, pwd string) error {
	log.WithContext(s.Ctx).Debugf("Auth %s", username)

	s.User = username

	infos := strings.Split(username, "@")
	if len(infos) > 1 {
		username = infos[0]
	}

	user, err := app.UserSvc.Authenticate(username, pwd)
	if err == nil && user != nil && user.ID > 0 {
		s.Ctx.UserAccount = user.Account
		s.Ctx.UserID = user.ID
		s.Ctx.UserName = user.Name
		s.Ctx.IsAdmin = user.IsAdmin == 1

		log.WithContext(s.Ctx).Debugf("Auth Success user_id=%d", user.ID)
		return nil
	}

	log.WithContext(s.Ctx).Debugf("登陆错误%s", username)
	return errors.New("password error")
}

func (s *SMTPSession) Mail(from string, opts *smtp.MailOptions) error {
	log.WithContext(s.Ctx).Debugf("Mail Success %+v %+v", from, opts)
	s.From = from
	return nil
}

func (s *SMTPSession) Rcpt(to string, opts *smtp.RcptOptions) error {
	log.WithContext(s.Ctx).Debugf("Rcpt Success %+v", to)

	s.To = append(s.To, to)
	return nil
}

func (s *SMTPSession) Reset() {}

func (s *SMTPSession) Logout() error {
	return nil
}
