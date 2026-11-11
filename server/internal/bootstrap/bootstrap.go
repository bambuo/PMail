package bootstrap

import (
	"os"
	"time"

	"fmt"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/internal/api"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/infra"
	"github.com/Jinnrry/pmail/pkg/file"
	log "github.com/sirupsen/logrus"

	// 新增：HTTP 就绪探测
	"net/http"
)

func Run(serverVersion string) {
	BindDependencies()
	go CertificateSchedulerStart()

	if !config.IsInit {
		lifecycleDirInit()

		go api.StartSetupHTTP()
		<-app.LifecycleInitChan
		api.StopSetupHTTP()
	}

	for {
		config.Init()
		// 移除：启动前立即更新证书的调用，避免 HTTP 未启动时触发 ACME
		// ssl.Update(false)

		parsemail.Init()
		err := infra.InitDatabase(serverVersion)
		if err != nil {
			panic(err)
		}
		infra.SessionInit()
		app.Sessions = infra.SessionInstance
		infra.PluginWasmInit(serverVersion)
		// smtp server start
		go api.StartSMTP()
		go api.StartSMTPS()
		go api.StartSubmissionSMTP()
		// http server start
		go api.StartHTTPS()
		go api.StartHTTP()

		// 等待 HTTP 服务就绪后再执行证书检查/续期，避免 ACME 验证失败
		lifecycleWaitHTTPReady()
		infra.

			// 启动后检查一遍证书（非重启模式）
			CertificateUpdate(false)

		// pop3 server start
		go api.StartPOP3()
		go api.StartPOP3S()
		// imap server start
		go api.StartIMAPS()

		log.Infof("Configuration loaded: database=%s http_port=%d", config.Instance.DbType, config.Instance.HttpPort)

		select {
		case <-app.LifecycleRestartChan:
			log.Infof("Server Restart!")
			api.StopSMTP()
			api.StopHTTPS()
			api.StopHTTP()
			api.StopPOP3()
			api.StopIMAP()
			infra.PluginWasmStop()
		case <-app.LifecycleStopChan:
			log.Infof("Server Stop!")
			api.StopSMTP()
			api.StopHTTPS()
			api.StopHTTP()
			api.StopPOP3()
			api.StopIMAP()
			infra.PluginWasmStop()
			return
		}
		log.Infof("Server Stop Success!")
		time.Sleep(5 * time.Second)

	}

}

func lifecycleDirInit() {
	if !file.PathExist("./config") {
		err := os.MkdirAll("./config", 0744)
		if err != nil {
			panic(err)
		}
	}

	if !file.PathExist("./config/dkim") {
		err := os.MkdirAll("./config/dkim", 0744)
		if err != nil {
			panic(err)
		}
	}

	if !file.PathExist("./config/ssl") {
		err := os.MkdirAll("./config/ssl", 0744)
		if err != nil {
			panic(err)
		}
	}
}

// 新增：HTTP 就绪探测（最多等待 ~90 秒）
func lifecycleWaitHTTPReady() {
	port := 80
	if config.Instance != nil && config.Instance.HttpPort > 0 {
		port = config.Instance.HttpPort
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/ping", port)

	for i := 0; i < 90; i++ {
		resp, err := http.Get(url)
		if err == nil && resp != nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Infof("HTTP ready: %s", url)
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	log.Warnf("HTTP not ready after 90s, skipping immediate SSL update")
}
