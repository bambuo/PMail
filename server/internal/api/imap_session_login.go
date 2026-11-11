package api

import (
	"database/sql"
	"strings"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/errors"
	"github.com/Jinnrry/pmail/pkg/password"
	"github.com/emersion/go-imap/v2"
	log "github.com/sirupsen/logrus"
)

func (s *imapServerSession) Login(username, pwd string) error {
	if strings.Contains(username, "@") {
		args := strings.Split(username, "@")
		username = args[0]
	}

	var user models.User

	_, err := app.Persistence.FindTokenAccount(username, &user)

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithContext(s.ctx).Errorf("%+v", err)
	}

	if user.ID > 0 && password.Verify(user.Password, pwd) {
		s.status = IMAPAUTHORIZED

		s.ctx.UserID = user.ID
		s.ctx.UserName = user.Name
		s.ctx.UserAccount = user.Account
		log.WithContext(s.ctx).Debug("Login successful")

		return nil
	}

	log.WithContext(s.ctx).Info("user not found")
	return &imap.Error{
		Type: imap.StatusResponseTypeNo,
		Code: imap.ResponseCodeAuthenticationFailed,
		Text: "Invalid credentials (Failure)",
	}
}
