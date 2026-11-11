package api

import (
	"strings"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/emersion/go-imap/v2"
)

func (s *imapServerSession) Rename(mailbox, newName string) error {
	if app.GroupIsDefaultBox(mailbox) {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: "This mailbox does not support rename.",
		}
	}

	groupPath := strings.Split(mailbox, "/")

	oldGroupName := groupPath[len(groupPath)-1]

	newGroupPath := strings.Split(newName, "/")

	newGroupName := newGroupPath[len(newGroupPath)-1]

	err := app.GroupRename(s.ctx, oldGroupName, newGroupName)

	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	return nil
}
