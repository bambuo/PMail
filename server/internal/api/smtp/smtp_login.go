package smtpapi

import "github.com/emersion/go-sasl"

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
