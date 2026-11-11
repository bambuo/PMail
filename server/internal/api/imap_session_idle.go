package api

import (
	"sync"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/emersion/go-imap/v2/imapserver"
	"github.com/spf13/cast"
)

type imapIdleConnection struct {
	writer  *imapserver.UpdateWriter
	mailbox string
}

type imapIdleConnections struct {
	mu      sync.Mutex
	writers map[string]imapIdleConnection
}

var imapUserConnects sync.Map
var imapUserConnectsMu sync.Mutex

func (s *imapServerSession) Idle(w *imapserver.UpdateWriter, stop <-chan struct{}) error {
	userId := s.ctx.UserID
	logId := cast.ToString(s.ctx.GetValue(context.LogID))

	imapUserConnectsMu.Lock()
	connectsAny, ok := imapUserConnects.Load(userId)
	if !ok {
		connectsAny = &imapIdleConnections{writers: map[string]imapIdleConnection{}}
		imapUserConnects.Store(userId, connectsAny)
	}
	connects := connectsAny.(*imapIdleConnections)

	connects.mu.Lock()
	connects.writers[logId] = imapIdleConnection{writer: w, mailbox: s.currentMailbox}
	connects.mu.Unlock()
	imapUserConnectsMu.Unlock()

	go func() {
		<-stop

		imapUserConnectsMu.Lock()
		connects.mu.Lock()
		delete(connects.writers, logId)
		if len(connects.writers) == 0 {
			imapUserConnects.Delete(userId)
		}
		connects.mu.Unlock()
		imapUserConnectsMu.Unlock()
	}()

	return nil
}

func IMAPIdleNotice(ctx *context.Context, userId int, email *models.Email) error {
	if userId == 0 || email == nil || email.Id == 0 {
		return nil
	}

	imapUserConnectsMu.Lock()
	connectsAny, ok := imapUserConnects.Load(userId)
	if !ok {
		imapUserConnectsMu.Unlock()
		return nil
	}

	connects := connectsAny.(*imapIdleConnections)
	connects.mu.Lock()
	idleConnects := make([]imapIdleConnection, 0, len(connects.writers))
	for _, connect := range connects.writers {
		idleConnects = append(idleConnects, connect)
	}
	connects.mu.Unlock()
	imapUserConnectsMu.Unlock()

	for _, connect := range idleConnects {
		connect.writer.WriteNumMessages(imapIdleNumMessages(ctx, userId, connect.mailbox))
	}
	return nil
}

func imapIdleNumMessages(ctx *context.Context, userId int, mailbox string) uint32 {
	if mailbox == "" {
		mailbox = "INBOX"
	}
	noticeCtx := *ctx
	noticeCtx.UserID = userId
	_, data := app.GetGroupStatus(&noticeCtx, mailbox, []string{"MESSAGES"})
	return cast.ToUint32(data["MESSAGES"])
}
