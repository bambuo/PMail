package imapapi

import (
	"strings"

	"github.com/emersion/go-imap/v2"
)

func (s *imapServerSession) Delete(mailbox string) error {
	groupPath := strings.Split(mailbox, "/")

	groupName := groupPath[len(groupPath)-1]
	groupInfo, err := s.server.groups.GetByName(s.ctx, groupName)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	_, err = s.server.groups.Delete(s.ctx, groupInfo.ID)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}

	return nil
}
