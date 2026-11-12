package app_test

import (
	"fmt"
	"testing"

	"github.com/Jinnrry/pmail/internal/app"

	"github.com/Jinnrry/pmail/internal/bootstrap"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/infra/persistence"
	"github.com/Jinnrry/pmail/pkg/context"
)

func TestGetGroupStatus(t *testing.T) {
	config.Init()
	engine, err := persistence.InitDatabase("")
	if err != nil {
		t.Fatal(err)
	}
	bootstrap.BindDatabase(engine)
	engine.
		ShowSQL(true)
	ctx := &context.Context{
		UserID:      1,
		UserName:    "admin",
		UserAccount: "admin",
	}
	ret, _ := app.GroupSvc.Status(ctx, "INBOX", []string{"MESSAGES", "UIDNEXT", "UIDVALIDITY", "UNSEEN"})
	fmt.Println(ret)
}
