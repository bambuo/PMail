package imapapi

import (
	"crypto/tls"
	"os"

	"github.com/Jinnrry/pmail/internal/config"
	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapserver"
	log "github.com/sirupsen/logrus"
)

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
