package api

import (
	"strings"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/emersion/go-imap/v2"
)

func (s *imapServerSession) Delete(mailbox string) error {
	groupPath := strings.Split(mailbox, "/")

	groupName := groupPath[len(groupPath)-1]
	groupInfo, err := app.GroupGetGroupByName(s.ctx, groupName)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	_, err = app.GroupDelGroup(s.ctx, groupInfo.ID)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}

	return nil
}
