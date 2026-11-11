package main

import (
	"log"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/api"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/bootstrap"
	"github.com/Jinnrry/pmail/internal/infra"
)

func main() {
	config.Init()
	if !config.IsInit {
		log.Fatal("e2e config not initialized")
	}
	parsemail.Init()
	if err := infra.InitDatabase("e2e"); err != nil {
		log.Fatal(err)
	}
	infra.SessionInit()
	app.Sessions = infra.SessionInstance
	bootstrap.BindDependencies()
	infra.PluginWasmInit("e2e")
	api.StartHTTP()
}
