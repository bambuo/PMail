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

// IMAPIdleNotifier 收信后向保持 IDLE 的连接推送 EXISTS 更新。
type IMAPIdleNotifier struct {
	groups *app.GroupService
}

func NewIMAPIdleNotifier(groups *app.GroupService) *IMAPIdleNotifier {
	return &IMAPIdleNotifier{groups: groups}
}

func (n *IMAPIdleNotifier) Notify(ctx *context.Context, userId int, email *models.Email) error {
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
		connect.writer.WriteNumMessages(cast.ToUint32(n.groups.MessageCount(ctx, userId, connect.mailbox)))
	}
	return nil
}
