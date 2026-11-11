package api

import (
	"crypto/rand"
	"crypto/tls"
	"time"

	"github.com/Jinnrry/gopop"
	"github.com/Jinnrry/pmail/config"
	log "github.com/sirupsen/logrus"
)

var pop3Instance *gopop.Server
var pop3InstanceTls *gopop.Server

func StartPOP3S() {
	crt, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		panic(err)
	}
	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{crt}
	tlsConfig.Time = time.Now
	tlsConfig.Rand = rand.Reader
	pop3InstanceTls = gopop.NewPop3Server(995, "pop."+config.Instance.Domain, true, tlsConfig, pop3Action{})
	pop3InstanceTls.ConnectAliveTime = 5 * time.Minute

	log.Infof("POP3 With TLS Server Start On Port :995")

	err = pop3InstanceTls.Start()
	if err != nil {
		panic(err)
	}
}

func StartPOP3() {
	crt, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		panic(err)
	}
	tlsConfig := &tls.Config{}
	tlsConfig.Certificates = []tls.Certificate{crt}
	tlsConfig.Time = time.Now
	tlsConfig.Rand = rand.Reader
	pop3Instance = gopop.NewPop3Server(110, "pop."+config.Instance.Domain, false, tlsConfig, pop3Action{})
	pop3Instance.ConnectAliveTime = 5 * time.Minute
	log.Infof("POP3 Server Start On Port :110")

	err = pop3Instance.Start()
	if err != nil {
		panic(err)
	}
}

func StopPOP3() {
	if pop3Instance != nil {
		pop3Instance.Stop()
	}

	if pop3InstanceTls != nil {
		pop3InstanceTls.Stop()
	}
}
