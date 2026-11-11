package api

import (
	"sync"
	"time"

	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/id"
	"github.com/emersion/go-imap/v2"
	log "github.com/sirupsen/logrus"

	"github.com/emersion/go-imap/v2/imapserver"
)

// Server is a server instance.
//
// A server contains a list of users.
type IMAPServer struct {
	mutex sync.Mutex
}

// NewServer creates a new server.
func IMAPNewServer() *IMAPServer {
	return &IMAPServer{}
}

type IMAPStatus int8

const (
	IMAPUNAUTHORIZED IMAPStatus = 1
	IMAPAUTHORIZED   IMAPStatus = 2
	IMAPSELECTED     IMAPStatus = 3
	IMAPLOGOUT       IMAPStatus = 4
)

type imapServerSession struct {
	server         *IMAPServer // immutable
	ctx            *context.Context
	status         IMAPStatus
	currentMailbox string
	connectTime    time.Time
	deleteUidList  []int
}

// NewSession creates a new IMAP session.
func (s *IMAPServer) NewSession() imapserver.Session {
	tc := &context.Context{}
	tc.SetValue(context.LogID, id.GenLogID())

	return &imapServerSession{
		server:      s,
		ctx:         tc,
		connectTime: time.Now(),
	}
}

func (s *imapServerSession) Close() error {
	return nil
}

func (s *imapServerSession) Subscribe(mailbox string) error {
	return nil
}

func (s *imapServerSession) Unsubscribe(mailbox string) error {
	return nil
}

func (s *imapServerSession) Append(mailbox string, r imap.LiteralReader, options *imap.AppendOptions) (*imap.AppendData, error) {
	log.WithContext(s.ctx).Errorf("Append Not Implemented")
	return nil, nil
}

func (s *imapServerSession) Unselect() error {
	s.currentMailbox = ""
	return nil
}
