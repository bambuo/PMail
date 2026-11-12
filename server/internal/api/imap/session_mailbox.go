package imapapi

import (
	"log/slog"
	"strings"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

func (s *imapServerSession) matchGroup(w *imapserver.ListWriter, basePath, pattern string) {
	var groups []*models.Group
	if basePath == "" && pattern == "*" {
		groups = s.server.groups.List(s.ctx)
		//w.WriteList(&imap.ListData{
		//	Attrs:   []imap.MailboxAttr{imap.MailboxAttrNoSelect, imap.MailboxAttrHasChildren},
		//	Delim:   '/',
		//	Mailbox: "[PMail]",
		//})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren},
			Delim:   '/',
			Mailbox: "INBOX",
		})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrSent},
			Delim:   '/',
			Mailbox: "Sent Messages",
		})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrDrafts},
			Delim:   '/',
			Mailbox: "Drafts",
		})

		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrTrash},
			Delim:   '/',
			Mailbox: "Deleted Messages",
		})
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrHasNoChildren, imap.MailboxAttrJunk},
			Delim:   '/',
			Mailbox: "Junk",
		})
	} else {
		pattern = strings.ReplaceAll(pattern, "/*", "/%")
		groups = s.server.groups.ListByPathPattern(s.ctx, pattern)
	}
	for _, group := range groups {

		data := &imap.ListData{
			Attrs:   []imap.MailboxAttr{},
			Mailbox: group.Name,
			Delim:   '/',
		}

		if s.server.groups.HasChildren(s.ctx, group.ID) {
			data.Attrs = append(data.Attrs, imap.MailboxAttrHasChildren)
		}

		data.Mailbox = s.server.groups.LayerName(s.ctx, group, true)

		w.WriteList(data)

	}

}

func (s *imapServerSession) List(w *imapserver.ListWriter, ref string, patterns []string, options *imap.ListOptions) error {
	log.WithContext(s.ctx).Debugf("imap server list, ref: %s ,patterns: %s ", ref, patterns)

	if ref == "" && len(patterns) == 0 {
		w.WriteList(&imap.ListData{
			Attrs:   []imap.MailboxAttr{imap.MailboxAttrNoSelect, imap.MailboxAttrHasChildren},
			Delim:   '/',
			Mailbox: "[PMail]",
		})
	}
	for _, pattern := range patterns {
		s.matchGroup(w, ref, pattern)
	}

	return nil
}

func (s *imapServerSession) Expunge(w *imapserver.ExpungeWriter, uids *imap.UIDSet) error {
	if uids == nil && len(s.deleteUidList) == 0 {
		return nil
	}
	uidList := []int{}

	if uids != nil {
		for _, uidRange := range *uids {
			if uidRange.Start > 0 && uidRange.Stop > 0 {
				for i := uidRange.Start; i <= uidRange.Stop; i++ {
					uidList = append(uidList, cast.ToInt(uint32(i)))
				}
			}
		}
	}

	if len(s.deleteUidList) > 0 {
		uidList = append(uidList, s.deleteUidList...)
	}

	if len(uidList) == 0 {
		return nil
	}

	slog.Debug("DeleteUidList:", slog.Any("uidList", uidList))

	err := s.server.emails.DeleteByUID(s.ctx, uidList)
	s.deleteUidList = []int{}
	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}

	return nil
}

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
