package imapapi

import (
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/emersion/go-imap/v2"
	"github.com/spf13/cast"
)

func (s *imapServerSession) Copy(numSet imap.NumSet, dest string) (*imap.CopyData, error) {

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

	if len(emailList) == 0 {
		return nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: "Email Not Found",
		}
	}

	var err error
	destUid := []int{}
	UIDValidity := 0
	if app.IsDefaultBox(dest) {
		UIDValidity, destUid, err = s.copyToDefaultBox(emailList, dest)
	} else {
		UIDValidity, destUid, err = s.copyToUserBox(emailList, dest)
	}
	data := imap.CopyData{}
	data.UIDValidity = cast.ToUint32(UIDValidity)
	data.DestUIDs = imap.UIDSet{}
	data.SourceUIDs = imap.UIDSet{}
	for _, uid := range destUid {
		data.DestUIDs = append(data.DestUIDs, imap.UIDRange{Start: imap.UID(cast.ToUint32(uid)), Stop: imap.UID(cast.ToUint32(uid))})
	}

	for _, email := range emailList {
		data.SourceUIDs = append(data.SourceUIDs, imap.UIDRange{Start: imap.UID(cast.ToUint32(email.UeId)), Stop: imap.UID(cast.ToUint32(email.UeId))})
	}

	return &data, err
}

func (s *imapServerSession) copyToDefaultBox(mails []*response.EmailResponseData, dest string) (int, []int, error) {
	return s.server.emails.CopyToDefaultBox(s.ctx, mails, dest)
}

func (s *imapServerSession) copyToUserBox(mails []*response.EmailResponseData, dest string) (int, []int, error) {
	groupInfo, err := s.server.groups.GetByFullPath(s.ctx, dest)
	if err != nil {
		return 0, nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	if groupInfo == nil || groupInfo.ID == 0 {
		return 0, nil, &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: "Group not found",
		}
	}
	return s.server.emails.CopyToGroup(s.ctx, mails, groupInfo.ID)
}
