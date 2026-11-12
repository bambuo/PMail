package imapapi

import (
	"strings"

	"github.com/emersion/go-imap/v2"
	log "github.com/sirupsen/logrus"
)

func (s *imapServerSession) Login(username, pwd string) error {
	if strings.Contains(username, "@") {
		args := strings.Split(username, "@")
		username = args[0]
	}

	user, err := s.server.users.Authenticate(username, pwd)
	if err == nil && user != nil && user.ID > 0 {
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
