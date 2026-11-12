package imapapi

import (
	"crypto/tls"
	"os"
	"sync"
	"time"

	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/id"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	log "github.com/sirupsen/logrus"
)

// Server is a server instance.
//
// A server contains a list of users.
type IMAPServer struct {
	mutex   sync.Mutex
	users   *app.UserService
	queries *app.EmailQueryService
	emails  *app.EmailService
	groups  *app.GroupService
}

// NewServer creates a new server.
func IMAPNewServer() *IMAPServer {
	return &IMAPServer{
		users:   app.UserSvc,
		queries: app.EmailQuerySvc,
		emails:  app.EmailSvc,
		groups:  app.GroupSvc,
	}
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

var imapInstanceTLS *imapserver.Server

func StopIMAP() {
	if imapInstanceTLS != nil {
		imapInstanceTLS.Close()
		imapInstanceTLS = nil
	}
}

// StarTLS 启动TLS端口监听，不加密的代码就懒得写了
func StartIMAPS() {
	imapInstanceTLS = imapNewIMAPServer()
	log.Infof("IMAP With TLS Server Start On Port :993")
	if err := imapInstanceTLS.ListenAndServeTLS(":993"); err != nil {
		panic(err)
	}
}

// NewTestServer 返回与生产 IMAP 监听相同配置的服务器实例，供 e2e 测试使用。
func NewTestServer() *imapserver.Server {
	return imapNewIMAPServer()
}

func imapNewIMAPServer() *imapserver.Server {
	crt, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		panic(err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{crt},
	}

	memServer := IMAPNewServer()

	option := &imapserver.Options{
		NewSession: func(conn *imapserver.Conn) (imapserver.Session, *imapserver.GreetingData, error) {
			return memServer.NewSession(), nil, nil
		},
		Caps: imap.CapSet{
			imap.CapIMAP4rev1: {},
			imap.CapIdle:      {},
		},
		TLSConfig:    tlsConfig,
		InsecureAuth: false,
	}

	if config.Instance.LogLevel == "debug" {
		option.DebugWriter = os.Stdout
	}

	return imapserver.New(option)
}
