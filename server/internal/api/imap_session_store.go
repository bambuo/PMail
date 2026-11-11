package api

import (
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/spf13/cast"
)

func (s *imapServerSession) Store(w *imapserver.FetchWriter, numSet imap.NumSet, flags *imap.StoreFlags, options *imap.StoreOptions) error {

	if flags.Op == imap.StoreFlagsSet {
		return nil
	}

	var emailList []*response.EmailResponseData

	switch numSet.(type) {
	case imap.SeqSet:
		seqSet := numSet.(imap.SeqSet)
		for _, seq := range seqSet {
			res := app.EmailListGetEmailListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(seq.Start),
				End:  cast.ToInt(seq.Stop),
			}, false)
			emailList = append(emailList, res...)
		}

	case imap.UIDSet:
		uidSet := numSet.(imap.UIDSet)
		for _, uid := range uidSet {
			res := app.EmailListGetEmailListByGroup(s.ctx, s.currentMailbox, app.EmailListImapListReq{
				Star: cast.ToInt(uint32(uid.Start)),
				End:  cast.ToInt(uint32(uid.Stop)),
			}, true)
			emailList = append(emailList, res...)
		}
	}

	if array.InArray(imap.FlagSeen, flags.Flags) && flags.Op == imap.StoreFlagsAdd {
		for _, data := range emailList {
			app.EmailDetailMakeRead(s.ctx, data.Id, flags.Op == imap.StoreFlagsAdd)
		}
	}

	if array.InArray(imap.FlagDeleted, flags.Flags) && flags.Op == imap.StoreFlagsAdd {
		for _, data := range emailList {
			s.deleteUidList = append(s.deleteUidList, data.UeId)
		}
	}

	err := app.EmailDeleteDelByUID(s.ctx, s.deleteUidList)
	s.deleteUidList = []int{}

	if err != nil {
		return &imap.Error{
			Type: imap.StatusResponseTypeNo,
			Text: err.Error(),
		}
	}

	return nil
}
