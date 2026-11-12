package imapapi

import (
	"strings"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
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

func (s *imapServerSession) Select(mailbox string, options *imap.SelectOptions) (*imap.SelectData, error) {
	if "" == mailbox {
		return nil, &imap.Error{
			Type: imap.StatusResponseTypeBad,
			Text: "mailbox not found",
		}
	}

	paths := strings.Split(mailbox, "/")
	s.currentMailbox = strings.Trim(paths[len(paths)-1], `"`)
	_, data := s.server.groups.Status(s.ctx, s.currentMailbox, []string{"MESSAGES", "UNSEEN", "UIDNEXT", "UIDVALIDITY"})

	ret := &imap.SelectData{
		Flags:          []imap.Flag{imap.FlagSeen},
		PermanentFlags: []imap.Flag{imap.FlagSeen},
		NumMessages:    cast.ToUint32(data["MESSAGES"]),
		UIDNext:        imap.UID(data["UIDNEXT"]),
		UIDValidity:    cast.ToUint32(data["UIDVALIDITY"]),
	}

	return ret, nil

}

func (s *imapServerSession) Status(mailbox string, options *imap.StatusOptions) (*imap.StatusData, error) {
	category := []string{}
	if options.UIDNext {
		category = append(category, "UIDNEXT")
	}
	if options.NumMessages {
		category = append(category, "MESSAGES")
	}
	if options.UIDValidity {
		category = append(category, "UIDVALIDITY")
	}
	if options.NumUnseen {
		category = append(category, "UNSEEN")
	}

	_, data := s.server.groups.Status(s.ctx, mailbox, category)

	numMessages := cast.ToUint32(data["MESSAGES"])
	numUnseen := cast.ToUint32(data["UNSEEN"])
	numValidity := cast.ToUint32(data["UIDVALIDITY"])
	numUIDNext := cast.ToUint32(data["UIDNEXT"])

	ret := &imap.StatusData{
		Mailbox:     mailbox,
		NumMessages: &numMessages,
		UIDNext:     imap.UID(numUIDNext),
		UIDValidity: numValidity,
		NumUnseen:   &numUnseen,
	}

	return ret, nil
}

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

func (s *imapServerSession) Rename(mailbox, newName string) error {
	if app.IsDefaultBox(mailbox) {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: "This mailbox does not support rename.",
		}
	}

	groupPath := strings.Split(mailbox, "/")

	oldGroupName := groupPath[len(groupPath)-1]

	newGroupPath := strings.Split(newName, "/")

	newGroupName := newGroupPath[len(newGroupPath)-1]

	err := s.server.groups.Rename(s.ctx, oldGroupName, newGroupName)

	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}
	return nil
}

func (s *imapServerSession) Namespace() (*imap.NamespaceData, error) {
	return &imap.NamespaceData{
		Personal: []imap.NamespaceDescriptor{
			{
				Prefix: "",
				Delim:  '/',
			},
		},
	}, nil
}

func (s *imapServerSession) Poll(w *imapserver.UpdateWriter, allowExpunge bool) error {

	ue := s.server.queries.RecentUserEmails(s.ctx, s.connectTime)
	if len(ue) > 0 {
		w.WriteNumMessages(cast.ToUint32(len(ue)))
	}

	return nil
}

func (s *imapServerSession) Store(w *imapserver.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {

	if flags.Op == imap.StoreFlagsSet {
		return nil
	}

	var emailList []*response.EmailResponseData

	switch numSet.(type) {
	case imap.SeqSet:
		seqSet := numSet.(imap.SeqSet)
		for _, seq := range seqSet {
			res := s.server.queries.ListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(seq.Start),
				End:  cast.ToInt(seq.Stop),
			}, false)
			emailList = append(emailList, res...)
		}

	case imap.UIDSet:
		uidSet := numSet.(imap.UIDSet)
		for _, uid := range uidSet {
			res := s.server.queries.ListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(uint32(uid.Start)),
				End:  cast.ToInt(uint32(uid.Stop)),
			}, true)
			emailList = append(emailList, res...)
		}
	}

	if array.InArray(imap.FlagSeen, flags.Flags) && flags.Op == imap.StoreFlagsAdd {
		for _, data := range emailList {
			s.server.emails.MarkRead(s.ctx, data.Id, flags.Op == imap.StoreFlagsAdd)
		}
	}

	if array.InArray(imap.FlagDeleted, flags.Flags) && flags.Op == imap.StoreFlagsAdd {
		for _, data := range emailList {
			s.deleteUidList = append(s.deleteUidList, data.UeId)
		}
	}

	err := s.server.emails.DeleteByUID(s.ctx, s.deleteUidList)
	s.deleteUidList = []int{}

	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}

	return nil
}
