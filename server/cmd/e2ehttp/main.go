package main

import (
	"log"

	"github.com/Jinnrry/pmail/dto/parsemail"
	httpapi "github.com/Jinnrry/pmail/internal/api/http"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/bootstrap"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/internal/infra/persistence"
	"github.com/Jinnrry/pmail/internal/infra/plugin"
	"github.com/Jinnrry/pmail/internal/infra/session"
)

func main() {
	config.Init()
	if !config.IsInit {
		log.Fatal("e2e config not initialized")
	}
	parsemail.Init()
	engine, err := persistence.InitDatabase("e2e")
	if err != nil {
		log.Fatal(err)
	}
	bootstrap.BindDatabase(engine)
	session.Init(engine)
	app.Sessions = session.Instance
	bootstrap.BindDependencies()
	plugin.WasmInit("e2e")
	httpapi.StartHTTP()
}
