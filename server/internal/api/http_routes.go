package api

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/internal/app"
	log "github.com/sirupsen/logrus"
)

// 这个服务是为了拦截http请求转发到https
var httpServerHttpServer *http.Server

func StopHTTP() {
	if httpServerHttpServer != nil {
		httpServerHttpServer.Close()
	}
}

func httpServerRouter(mux *http.ServeMux) {
	fe, err := fs.Sub(httpServerLocal, "dist")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(fe)))
	// 挑战请求类似这样 /.well-known/acme-challenge/QPyMAyaWw9s5JvV1oruyqWHG7OqkHMJEHPoUz2046KM
	mux.HandleFunc("/.well-known/", HTTPAcmeChallenge)
	mux.HandleFunc("/api/ping", HTTPPing)
	mux.HandleFunc("/api/login", httpServerContextIterceptor(HTTPLogin))
	mux.HandleFunc("/api/logout", httpServerContextIterceptor(HTTPLogout))
	mux.HandleFunc("/api/group", httpServerContextIterceptor(HTTPGetUserGroup))
	mux.HandleFunc("/api/group/list", httpServerContextIterceptor(HTTPGetUserGroupList))
	mux.HandleFunc("/api/group/add", httpServerContextIterceptor(HTTPAddGroup))
	mux.HandleFunc("/api/group/del", httpServerContextIterceptor(HTTPDelGroup))
	mux.HandleFunc("/api/email/list", httpServerContextIterceptor(EmailHTTPEmailList))
	mux.HandleFunc("/api/email/del", httpServerContextIterceptor(EmailHTTPEmailDelete))
	mux.HandleFunc("/api/email/read", httpServerContextIterceptor(EmailHTTPMarkRead))
	mux.HandleFunc("/api/email/detail", httpServerContextIterceptor(EmailHTTPEmailDetail))
	mux.HandleFunc("/api/email/move", httpServerContextIterceptor(EmailHTTPMove))
	mux.HandleFunc("/api/email/send", httpServerContextIterceptor(EmailHTTPSend))
	mux.HandleFunc("/api/settings/modify_password", httpServerContextIterceptor(HTTPModifyPassword))
	mux.HandleFunc("/api/rule/get", httpServerContextIterceptor(HTTPGetRule))
	mux.HandleFunc("/api/rule/add", httpServerContextIterceptor(HTTPUpsertRule))
	mux.HandleFunc("/api/rule/update", httpServerContextIterceptor(HTTPUpsertRule))
	mux.HandleFunc("/api/rule/del", httpServerContextIterceptor(HTTPDelRule))
	mux.HandleFunc("/attachments/", httpServerContextIterceptor(HTTPGetAttachments))
	mux.HandleFunc("/attachments/download/", httpServerContextIterceptor(HTTPDownload))
	mux.HandleFunc("/api/user/create", httpServerContextIterceptor(HTTPCreateUser))
	mux.HandleFunc("/api/user/edit", httpServerContextIterceptor(HTTPEditUser))
	mux.HandleFunc("/api/user/info", httpServerContextIterceptor(HTTPInfo))
	mux.HandleFunc("/api/user/list", httpServerContextIterceptor(HTTPUserList))
	mux.HandleFunc("/api/plugin/settings/", httpServerContextIterceptor(HTTPSettingsHtml))
	mux.HandleFunc("/api/plugin/list", httpServerContextIterceptor(HTTPGetPluginList))
	mux.HandleFunc("/api/plugin/manage/list", httpServerContextIterceptor(HTTPPluginManageList))
	mux.HandleFunc("/api/plugin/manage/approve", httpServerContextIterceptor(HTTPPluginManageApprove))
	mux.HandleFunc("/api/plugin/manage/enable", httpServerContextIterceptor(HTTPPluginManageEnable))
	mux.HandleFunc("/api/plugin/manage/disable", httpServerContextIterceptor(HTTPPluginManageDisable))
	mux.HandleFunc("/api/plugin/manage/reject", httpServerContextIterceptor(HTTPPluginManageReject))
	mux.HandleFunc("/api/plugin/manage/reload", httpServerContextIterceptor(HTTPPluginManageReload))
}

func StartHTTP() {
	mux := http.NewServeMux()

	HttpPort := 80
	if config.Instance.HttpPort > 0 {
		HttpPort = config.Instance.HttpPort
	}

	if config.Instance.HttpsEnabled != 2 {
		// 在重定向模式下，也必须显式处理 ACME 挑战，避免跳转导致验证失败
		mux.HandleFunc("/.well-known/", HTTPAcmeChallenge)
		mux.HandleFunc("/api/ping", HTTPPing)
		mux.HandleFunc("/", HTTPInterceptor)
		httpServerHttpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", HttpPort),
			Handler:      mux,
			ReadTimeout:  time.Second * 90,
			WriteTimeout: time.Second * 90,
		}
	} else {

		httpServerRouter(mux)

		log.Infof("HttpServer Start On Port :%d", HttpPort)
		httpServerHttpServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", HttpPort),
			Handler:      app.Sessions.LoadAndSave(mux),
			ReadTimeout:  time.Second * 90,
			WriteTimeout: time.Second * 90,
		}
	}

	err := httpServerHttpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
