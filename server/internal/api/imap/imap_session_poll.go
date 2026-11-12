package imapapi

import (
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/spf13/cast"
)

func (s *imapServerSession) Poll(w *imapserver.UpdateWriter, allowExpunge bool) error {

	ue := s.server.queries.RecentUserEmails(s.ctx, s.connectTime)
	if len(ue) > 0 {
		w.WriteNumMessages(cast.ToUint32(len(ue)))
	}

	return nil
}
