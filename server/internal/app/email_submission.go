package app

import (
	"net"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
)

type MailSubmission struct {
	RemoteAddress net.Addr
	User          string
	From          string
	To            []string
	Ctx           *context.Context
}

var NotifyMailbox func(*context.Context, int, *models.Email) error
var CheckSPF func(string, *parsemail.User, string) bool
var PersistInitialAudit func(*context.Context, models.Email, []int) (*models.Email, error)
var PersistDeliveryResult func(*context.Context, int, int8, any) error
var FindRecipientAccounts func([]string, *[]*models.User) error

var DeliveryResultError func(error, map[string]error) error
