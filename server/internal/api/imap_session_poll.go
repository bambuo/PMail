package api

import (
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/spf13/cast"
)

func (s *imapServerSession) Poll(w *imapserver.UpdateWriter, allowExpunge bool) error {

	var ue []models.UserEmail
	app.Persistence.FindRecentUserEmails(s.ctx.UserID, s.connectTime, &ue)

	if len(ue) > 0 {
		w.WriteNumMessages(cast.ToUint32(len(ue)))
	}

	return nil
}
