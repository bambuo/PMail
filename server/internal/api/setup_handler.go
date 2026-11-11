package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

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

func HTTPSetup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
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
		dbType, dbDSN, err := app.SetupGetDatabaseSettings(ctx)
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
		err := app.SetupSetDatabaseSettings(ctx, cast.ToString(reqData["db_type"]), cast.ToString(reqData["db_dsn"]))
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}

		response.NewSuccessResponse("Succ").FPrint(w)
		return
	}

	if reqData["step"] == "password" && reqData["action"] == "get" {
		ok, err := app.SetupGetAdminPassword(ctx)
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse(ok).FPrint(w)
		return
	}

	if reqData["step"] == "password" && reqData["action"] == "set" {
		err := app.SetupSetAdminPassword(ctx, cast.ToString(reqData["account"]), cast.ToString(reqData["password"]))
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse("Succ").FPrint(w)
		return
	}

	if reqData["step"] == "domain" && reqData["action"] == "get" {
		smtpDomain, webDomain, domains, err := app.SetupGetDomainSettings()
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
		err := app.SetupSetDomainSettings(cast.ToString(reqData["smtp_domain"]), cast.ToString(reqData["web_domain"]), reqData["multi_domain"])
		if err != nil {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
			return
		}
		response.NewSuccessResponse("Succ").FPrint(w)
		return
	}

	if reqData["step"] == "dns" && reqData["action"] == "get" {
		dnsInfos, err := app.SetupGetDNSSettings(ctx)
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
			app.SetupFinish()
		}
		return
	}

}
