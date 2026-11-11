package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/password"
	log "github.com/sirupsen/logrus"
)

type httpModifyPasswordRequest struct {
	Password string `json:"password"`
}

func HTTPModifyPassword(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var retData httpModifyPasswordRequest
	err = json.Unmarshal(reqBytes, &retData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	if retData.Password != "" {
		encodePwd := password.Hash(retData.Password)

		_, err := app.Persistence.SetAccountPassword(ctx.UserID, encodePwd)
		if err != nil {
			response.NewErrorResponse(response.ServerError, app.TextGetText(ctx.Lang, "unknowError"), "").FPrint(w)
			return
		}

	}

	response.NewSuccessResponse(app.TextGetText(ctx.Lang, "succ")).FPrint(w)
}
