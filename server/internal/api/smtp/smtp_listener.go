package smtpapi

import (
	"crypto/tls"
	"time"

	"github.com/Jinnrry/pmail/internal/config"
	"github.com/emersion/go-smtp"
	log "github.com/sirupsen/logrus"
)

var smtpInstance *smtp.Server
var smtpInstanceTls *smtp.Server
var smtpInstanceTlsNew *smtp.Server

func StartSubmissionSMTP() {
	be := &SMTPBackend{}

	smtpInstanceTlsNew = smtp.NewServer(be)

	smtpInstanceTlsNew.Addr = ":587"
	smtpInstanceTlsNew.Domain = config.Instance.Domain
	smtpInstanceTlsNew.ReadTimeout = 10 * time.Second
	smtpInstanceTlsNew.WriteTimeout = 10 * time.Second
	smtpInstanceTlsNew.MaxMessageBytes = 1024 * 1024 * 30
	smtpInstanceTlsNew.MaxRecipients = 50
	// force TLS for auth
	smtpInstanceTlsNew.AllowInsecureAuth = true
	// Load the certificate and key
	cer, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		log.Fatal(err)
		return
	}
	// Configure the TLS support for STARTTLS
	smtpInstanceTlsNew.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}

	log.Println("Starting Smtp With STARTTLS Server Port:", smtpInstanceTlsNew.Addr)
	// 587端口使用STARTTLS（先明文连接，再升级TLS），而非隐式TLS
	if err := smtpInstanceTlsNew.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func StartSMTPS() {
	be := &SMTPBackend{}

	smtpInstanceTls = smtp.NewServer(be)

	smtpInstanceTls.Addr = ":465"
	smtpInstanceTls.Domain = config.Instance.Domain
	smtpInstanceTls.ReadTimeout = 10 * time.Second
	smtpInstanceTls.WriteTimeout = 10 * time.Second
	smtpInstanceTls.MaxMessageBytes = 1024 * 1024 * 30
	smtpInstanceTls.MaxRecipients = 50
	// force TLS for auth
	smtpInstanceTls.AllowInsecureAuth = true
	// Load the certificate and key
	cer, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		log.Fatal(err)
		return
	}
	// Configure the TLS support
	smtpInstanceTls.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}

	log.Println("Starting Smtp With SSL Server Port:", smtpInstanceTls.Addr)
	if err := smtpInstanceTls.ListenAndServeTLS(); err != nil {
		log.Fatal(err)
	}
}

func StartSMTP() {
	be := &SMTPBackend{}

	smtpInstance = smtp.NewServer(be)

	smtpInstance.Addr = ":25"
	smtpInstance.Domain = config.Instance.Domain
	smtpInstance.ReadTimeout = 10 * time.Second
	smtpInstance.WriteTimeout = 10 * time.Second
	smtpInstance.MaxMessageBytes = 1024 * 1024 * 30
	smtpInstance.MaxRecipients = 50
	// force TLS for auth
	smtpInstance.AllowInsecureAuth = false
	// Load the certificate and key
	cer, err := tls.LoadX509KeyPair(config.Instance.SSLPublicKeyPath, config.Instance.SSLPrivateKeyPath)
	if err != nil {
		log.Fatal(err)
		return
	}
	// Configure the TLS support
	smtpInstance.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cer}}

	log.Println("Starting Smtp Server Port:", smtpInstance.Addr)
	if err := smtpInstance.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

func StopSMTP() {
	if smtpInstance != nil {
		smtpInstance.Close()
	}
	if smtpInstanceTls != nil {
		smtpInstanceTls.Close()
	}

	if smtpInstanceTlsNew != nil {
		smtpInstanceTlsNew.Close()
	}
}
