package api

import (
	"strings"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/emersion/go-imap/v2"
)

func (s *imapServerSession) Create(mailbox string, options *imap.CreateOptions) error {
	groupPath := strings.Split(mailbox, "/")

	var parentId int
	for _, path := range groupPath {
		newGroup, err := app.CreateGroup(s.ctx, path, parentId)
		if err != nil {
			return &imap.Error{
				Type: imap.StatusResponseTypeNo,
				Text: err.Error(),
			}
		}
		parentId = newGroup.ID
	}

	return nil
}
