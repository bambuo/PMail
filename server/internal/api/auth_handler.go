package api

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/errors"
	"github.com/Jinnrry/pmail/pkg/password"
	log "github.com/sirupsen/logrus"
)

type httpLoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

func HTTPLogin(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var reqData httpLoginRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	var user models.User

	encodePwd := password.Encode(reqData.Password)
	_, err = app.Persistence.FindLoginAccount(reqData.Account, encodePwd, &user)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Errorf("%+v", err)
	}

	if user.ID != 0 {
		if app.MFAIsEnabled(&user) {
			token, err := app.CreateMFAChallenge(user.ID)
			if err != nil {
				response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
				return
			}
			response.NewSuccessResponse(map[string]any{
				"mfaRequired":    true,
				"challengeToken": token,
			}).FPrint(w)
			return
		}
		userStr, _ := json.Marshal(user)
		app.Sessions.Put(req.Context(), "user", string(userStr))
		loginUserResponse(w, &user)
	} else {
		response.NewErrorResponse(response.ParamsError, app.TextGetText(ctx.Lang, "aperror"), "").FPrint(w)
	}
}

func loginUserResponse(w http.ResponseWriter, user *models.User) {
	domains := config.Instance.Domains
	domains = array.Difference(domains, []string{config.Instance.Domain})
	domains = append([]string{config.Instance.Domain}, domains...)

	response.NewSuccessResponse(map[string]any{
		"account":  user.Account,
		"name":     user.Name,
		"is_admin": user.IsAdmin,
		"domains":  domains,
	}).FPrint(w)
}

func HTTPLogout(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	app.Sessions.Clear(ctx.Context)
	response.NewSuccessResponse("Success").FPrint(w)
}
