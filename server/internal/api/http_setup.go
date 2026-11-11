package api

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/pkg/ip"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

// 项目初始化引导用的服务，初始化引导结束后即退出
var httpServerSetupServer *http.Server

func StartSetupHTTP() {
	mux := http.NewServeMux()
	fe, err := fs.Sub(httpServerLocal, "dist")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", httpServerSPAHandler(fe))
	mux.HandleFunc("/api/", httpServerContextIterceptor(HTTPSetup))
	// 挑战请求类似这样 /.well-known/acme-challenge/QPyMAyaWw9s5JvV1oruyqWHG7OqkHMJEHPoUz2046KM
	mux.HandleFunc("/.well-known/", HTTPAcmeChallenge)

	HttpPort := 80
	flag.IntVar(&HttpPort, "p", 80, "初始化阶段Http服务端口")
	flag.Parse()

	if HttpPort == 80 {
		HttpPort = cast.ToInt(os.Getenv("setup_port"))
	}

	if HttpPort <= 0 || HttpPort > 65535 {
		HttpPort = 80
	}

	config.Instance.SetSetupPort(HttpPort)
	log.Infof("HttpServer Start On Port :%d", HttpPort)
	if HttpPort == 80 {
		log.Infof("Please click http://%s to continue.\n", ip.GetIp())
	} else {
		log.Infof("Please click http://%s:%d to continue.", ip.GetIp(), HttpPort)
	}

	httpServerSetupServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", HttpPort),
		Handler:      mux,
		ReadTimeout:  time.Second * 60,
		WriteTimeout: time.Second * 60,
	}
	err = httpServerSetupServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

func StopSetupHTTP() {
	err := httpServerSetupServer.Close()
	log.Infof("Setup End!")
	if err != nil {
		panic(err)
	}
}
