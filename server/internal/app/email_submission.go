package app

import (
	"net"

	"github.com/Jinnrry/pmail/pkg/context"
)

// MailSubmission SMTP 会话的用例载体：协议字段 + 邮件写服务。
type MailSubmission struct {
	RemoteAddress net.Addr
	User          string
	From          string
	To            []string
	Ctx           *context.Context
	Service       *EmailService
}
