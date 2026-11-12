package main

import (
	"github.com/Jinnrry/pmail/internal/bootstrap"
	"github.com/Jinnrry/pmail/internal/config"
	log "github.com/sirupsen/logrus"
)

var (
	gitHash   string
	buildTime string
	goVersion string
	version   string
)

func main() {

	config.Init()

	if version == "" {
		version = "TestVersion"
	}

	log.Infoln("*******************************************************************")
	log.Infof("***\tServer Start Success \n")
	log.Infof("***\tServer Version: %s \n", version)
	log.Infof("***\tGit Commit Hash: %s ", gitHash)
	log.Infof("***\tBuild Date: %s ", buildTime)
	log.Infof("***\tBuild GoLang Version: %s ", goVersion)
	log.Infoln("*******************************************************************")

	// 定时任务启动
	// Scheduled tasks are started by the composition root.
	bootstrap.

		// 核心服务启动
		Run(version)

	log.Warnf("Server Stoped \n")

}
