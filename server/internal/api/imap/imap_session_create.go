package imapapi

import (
	"strings"

	"github.com/emersion/go-imap/v2"
)

func (s *imapServerSession) Create(mailbox string, options *imap.CreateOptions) error {
	groupPath := strings.Split(mailbox, "/")

	var parentId int
	for _, path := range groupPath {
		newGroup, err := s.server.groups.Create(s.ctx, path, parentId)
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
