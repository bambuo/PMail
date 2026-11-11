package api

import (
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
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
			emailList = app.EmailListGetEmailListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(seq.Start),
				End:  cast.ToInt(seq.Stop),
			}, false)
		}
	case imap.UIDSet:
		uidSet := numSet.(imap.UIDSet)
		for _, uid := range uidSet {
			emailList = app.EmailListGetEmailListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
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

	if app.GroupIsDefaultBox(dest) {
		return imapMove2defaultbox(s.ctx, mailIds, dest)
	} else {
		return imapMove2userbox(s.ctx, mailIds, dest)
	}

}

func imapMove2defaultbox(ctx *context.Context, mailIds []int, dest string) error {
	err := app.GroupMove2DefaultBox(ctx, mailIds, dest)
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	return nil
}

func imapMove2userbox(ctx *context.Context, mailIds []int, dest string) error {
	groupInfo, err := app.GroupGetGroupByFullPath(ctx, dest)
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
	app.GroupMoveMailToGroup(ctx, mailIds, groupInfo.ID)

	return nil
}
