package httpapi

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/pkg/context"
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
	mux.HandleFunc("/api/", httpServerContextIterceptor((&SetupHandler{setup: app.SetupSvc}).Setup))
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

func HTTPAcmeChallenge(w http.ResponseWriter, r *http.Request) {
	log.Infof("AcmeChallenge: %s", r.URL.Path)

	token := strings.ReplaceAll(r.URL.Path, "/.well-known/acme-challenge/", "")
	keyAuth, exist := app.HTTPChallenge(token)
	if exist {
		w.Write([]byte(keyAuth))
	} else {
		log.Error("ACME challenge token not found")
		http.NotFound(w, r)
	}
}

type httpSslResponse struct {
	Port int    `json:"port"`
	Type string `json:"type"`
}

// SetupHandler 初始化引导接口，委托初始化用例服务
type SetupHandler struct {
	setup *app.SetupService
}

func (h *SetupHandler) Setup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		response.NewSuccessResponse("").FPrint(w)
		return
	}

	var reqData map[string]string
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		response.NewErrorResponse(response.ServerError, "", err.Error()).FPrint(w)
		return
	}

	if reqData["step"] == "database" && reqData["action"] == "get" {
		dbType, dbDSN, err := h.setup.DatabaseSettings(ctx)
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}

		response.NewSuccessResponse(map[string]string{
			"db_type": dbType,
			"db_dsn":  dbDSN,
		}).FPrint(w)
		return
	}

	if reqData["step"] == "database" && reqData["action"] == "set" {
		err := h.setup.SetDatabaseSettings(ctx, cast.ToString(reqData["db_type"]), cast.ToString(reqData["db_dsn"]))
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}

		response.NewSuccessResponse("Succ").FPrint(w)
		return
	}

	if reqData["step"] == "password" && reqData["action"] == "get" {
		ok, err := h.setup.AdminPassword(ctx)
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse(ok).FPrint(w)
		return
	}

	if reqData["step"] == "password" && reqData["action"] == "set" {
		err := h.setup.SetAdminPassword(ctx, cast.ToString(reqData["account"]), cast.ToString(reqData["password"]))
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse("Succ").FPrint(w)
		return
	}

	if reqData["step"] == "domain" && reqData["action"] == "get" {
		smtpDomain, webDomain, domains, err := h.setup.DomainSettings()
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse(map[string]any{
			"smtp_domain": smtpDomain,
			"web_domain":  webDomain,
			"domains":     domains,
		}).FPrint(w)
		return
	}

	if reqData["step"] == "domain" && reqData["action"] == "set" {
		err := h.setup.SetDomainSettings(cast.ToString(reqData["smtp_domain"]), cast.ToString(reqData["web_domain"]), reqData["multi_domain"])
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse("Succ").FPrint(w)
		return
	}

	if reqData["step"] == "dns" && reqData["action"] == "get" {
		dnsInfos, err := h.setup.DNSSettings(ctx)
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse(dnsInfos).FPrint(w)
		return
	}

	if reqData["step"] == "ssl" && reqData["action"] == "get" {
		sslType := app.GetCertificateType()
		res := httpSslResponse{
			Type: sslType,
			Port: config.Instance.GetSetupPort(),
		}
		response.NewSuccessResponse(res).FPrint(w)
		return
	}

	if reqData["step"] == "ssl" && reqData["action"] == "getParams" {

		response.NewSuccessResponse(app.GetDNSChallengeRecords(ctx)).FPrint(w)
		return
	}

	if reqData["step"] == "ssl" && reqData["action"] == "set" {

		if reqData["ssl_type"] == config.SSLTypeUser {
			keyPath := reqData["key_path"]
			crtPath := reqData["crt_path"]

			_, err := os.Stat(cast.ToString(keyPath))
			if err != nil {
				response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
				return
			}

			_, err = os.Stat(cast.ToString(crtPath))
			if err != nil {
				response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
				return
			}
		}

		err = app.SetCertificate(cast.ToString(reqData["ssl_type"]), cast.ToString(reqData["key_path"]), cast.ToString(reqData["crt_path"]))
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}

		if reqData["ssl_type"] == config.SSLTypeAutoHTTP || reqData["ssl_type"] == config.SSLTypeAutoDNS {
			err = app.GenerateCertificate(false)
			if err != nil {
				response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
				return
			}
		}

		response.NewSuccessResponse("Succ").FPrint(w)

		if reqData["ssl_type"] == config.SSLTypeUser {
			h.setup.Finish()
		}
		return
	}

}
