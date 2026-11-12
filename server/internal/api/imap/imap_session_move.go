package imapapi

import (
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/spf13/cast"
)

func (s *imapServerSession) Move(w *imapserver.MoveWriter, numSet imap.NumSet, dest string) error {

	var emailList []*response.EmailResponseData

	switch numSet.(type) {
	case imap.SeqSet:
		seqSet := numSet.(imap.SeqSet)
		for _, seq := range seqSet {
			emailList = s.server.queries.ListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(seq.Start),
				End:  cast.ToInt(seq.Stop),
			}, false)
		}
	case imap.UIDSet:
		uidSet := numSet.(imap.UIDSet)
		for _, uid := range uidSet {
			emailList = s.server.queries.ListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(uint32(uid.Start)),
				End:  cast.ToInt(uint32(uid.Stop)),
			}, true)
		}
	}

	var mailIds []int
	for _, email := range emailList {
		mailIds = append(mailIds, email.Id)
	}

	if mailIds == nil || len(mailIds) == 0 {
		return nil
	}

	if app.IsDefaultBox(dest) {
		return s.moveToDefaultBox(mailIds, dest)
	}
	return s.moveToUserBox(mailIds, dest)
}

func (s *imapServerSession) moveToDefaultBox(mailIds []int, dest string) error {
	err := s.server.emails.MoveToDefaultBox(s.ctx, mailIds, dest)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	return nil
}

func (s *imapServerSession) moveToUserBox(mailIds []int, dest string) error {
	groupInfo, err := s.server.groups.GetByFullPath(s.ctx, dest)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	if groupInfo == nil || groupInfo.ID == 0 {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: "Group not found",
		}
	}
	s.server.emails.MoveToGroup(s.ctx, mailIds, groupInfo.ID)

	return nil
}
