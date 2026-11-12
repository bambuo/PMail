package smtpapi

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/textproto"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/emersion/go-sasl"
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

// Authenticates users with an username and a password.
type SMTPLoginAuthenticator func(username, password string) error

type smtpLoginState int

const (
	smtpLoginNotStarted smtpLoginState = iota
	smtpLoginWaitingUsername
	smtpLoginWaitingPassword
)

type smtpLoginServer struct {
	state              smtpLoginState
	username, password string
	authenticate       SMTPLoginAuthenticator
}

// A server implementation of the LOGIN authentication mechanism, as described
// in https://tools.ietf.org/html/draft-murchison-sasl-login-00.
//
// LOGIN is obsolete and should only be enabled for legacy clients that cannot
// be updated to use PLAIN.
func SMTPNewLoginServer(authenticator SMTPLoginAuthenticator) sasl.Server {
	return &smtpLoginServer{authenticate: authenticator}
}

func (a *smtpLoginServer) Next(response []byte) (challenge []byte, done bool, err error) {
	switch a.state {
	case smtpLoginNotStarted:
		// Check for initial response field, as per RFC4422 section 3
		if response == nil {
			challenge = []byte("Username:")
			break
		}
		a.state++
		fallthrough
	case smtpLoginWaitingUsername:
		a.username = string(response)
		challenge = []byte("Password:")
	case smtpLoginWaitingPassword:
		a.password = string(response)
		err = a.authenticate(a.username, a.password)
		done = true
	default:
		err = sasl.ErrUnexpectedClientResponse
	}

	a.state++
	return
}

func (s *SMTPSession) Data(r io.Reader) error { return (*app.MailSubmission)(s).Receive(r) }
func (s *SMTPSession) deliverOutgoingEmail(email *parsemail.Email, sender func(*context.Context, *parsemail.Email) (error, map[string]error)) error {
	return (*app.MailSubmission)(s).DeliverOutgoingEmail(email, sender)
}

const smtpMaxDeliverySMTPMessageBytes = 128

// downstreamDeliverySMTPError turns the synchronous downstream delivery result
// into the SMTP response returned to the submitting client. Unknown failures are
// temporary so a transient network problem does not permanently drop a message.
func smtpDownstreamDeliverySMTPError(deliveryErr error, domainErrors map[string]error) error {
	if deliveryErr == nil {
		return nil
	}

	permanent := len(domainErrors) > 0
	for _, domainErr := range domainErrors {
		if domainErr == nil || !smtpIsPermanentDeliveryError(domainErr) {
			permanent = false
			break
		}
	}

	if permanent {
		return &smtp.SMTPError{
			Code:         550,
			EnhancedCode: smtp.EnhancedCode{5, 0, 0},
			Message:      smtpDeliverySMTPMessage("Downstream delivery permanently failed", deliveryErr, domainErrors),
		}
	}

	return &smtp.SMTPError{
		Code:         451,
		EnhancedCode: smtp.EnhancedCode{4, 4, 0},
		Message:      smtpDeliverySMTPMessage("Downstream delivery temporarily failed", deliveryErr, domainErrors),
	}
}

func smtpDeliverySMTPMessage(prefix string, deliveryErr error, domainErrors map[string]error) string {
	details := make([]string, 0, len(domainErrors))
	domains := make([]string, 0, len(domainErrors))
	for domain := range domainErrors {
		domains = append(domains, domain)
	}
	sort.Strings(domains)
	for _, domain := range domains {
		if domainErr := domainErrors[domain]; domainErr != nil {
			details = append(details, domain+": "+domainErr.Error())
		}
	}
	if len(details) == 0 && deliveryErr != nil {
		details = append(details, deliveryErr.Error())
	}

	message := prefix
	if len(details) > 0 {
		// SMTP replies must stay on one bounded line, even when a remote server
		// returns malformed text. Retain useful detail for the submitting client.
		message += ": " + strings.Join(strings.Fields(strings.Join(details, "; ")), " ")
	}
	if len(message) <= smtpMaxDeliverySMTPMessageBytes {
		return message
	}

	limit := smtpMaxDeliverySMTPMessageBytes - len("...")
	for len(message) > limit {
		_, size := utf8.DecodeLastRuneInString(message)
		message = message[:len(message)-size]
	}
	return message + "..."
}

func smtpIsPermanentDeliveryError(err error) bool {
	var protocolErr *textproto.Error
	if errors.As(err, &protocolErr) {
		return protocolErr.Code >= 500 && protocolErr.Code <= 599
	}

	var smtpErr *smtp.SMTPError
	if errors.As(err, &smtpErr) {
		return smtpErr.Code >= 500 && smtpErr.Code <= 599
	}

	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func DeliveryResultError(err error, details map[string]error) error {
	return smtpDownstreamDeliverySMTPError(err, details)
}
