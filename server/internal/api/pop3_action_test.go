package api

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/Jinnrry/gopop"
	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/emersion/go-message/mail"
)

func Test_action_Retr(t *testing.T) {
	config.Init()
	config.Instance.DbType = config.DBTypeSQLite
	config.Instance.DbDSN = config.ROOT_PATH + "./config/pmail_temp.db"
	infra.InitDatabase("")

	a := pop3Action{}
	session := &gopop.Session{
		Ctx: &context.Context{
			UserID: 1,
		},
	}
	got, got1, err := a.Retr(session, 301)

	_, _, _ = got, got1, err
}

func Test_email(t *testing.T) {
	var b bytes.Buffer

	// Create our mail header
	var h mail.Header

	// Create a new mail writer
	mw, _ := mail.CreateWriter(&b, h)

	// Create a text part
	tw, _ := mw.CreateInline()

	var html mail.InlineHeader

	html.Header.Set("Content-Transfer-Encoding", "base64")
	w, _ := tw.CreatePart(html)

	io.WriteString(w, "=")

	w.Close()

	tw.Close()

	fmt.Printf("%s", b.String())

}
