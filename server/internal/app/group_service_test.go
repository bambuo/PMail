package app_test

import (
	"fmt"
	"testing"

	"github.com/Jinnrry/pmail/internal/app"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/pkg/context"
)

func TestGetGroupStatus(t *testing.T) {
	config.Init()
	infra.InitDatabase("")
	infra.Database.
		ShowSQL(true)
	ctx := &context.Context{
		UserID:      1,
		UserName:    "admin",
		UserAccount: "admin",
	}
	ret, _ := app.GetGroupStatus(ctx, "INBOX", []string{"MESSAGES", "UIDNEXT", "UIDVALIDITY", "UNSEEN"})
	fmt.Println(ret)
}
