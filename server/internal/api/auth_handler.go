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

	_, err = app.Persistence.FindTokenAccount(reqData.Account, &user)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Errorf("%+v", err)
	}

	if user.ID != 0 && password.Verify(user.Password, reqData.Password) {
		// 历史双重 MD5 哈希在登录成功后透明升级为 bcrypt
		if password.NeedsUpgrade(user.Password) {
			if _, err := app.Persistence.SetAccountPassword(user.ID, password.Hash(reqData.Password)); err != nil {
				log.WithContext(ctx).Errorf("upgrade password hash failed: %+v", err)
			}
		}
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
		// 用于按需铸造 5 秒时效 Token 的口令哈希（md5(api_key+timestamp)），仅返回给完成登录的用户本人
		"api_key": user.Password,
	}).FPrint(w)
}

func HTTPLogout(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	app.Sessions.Clear(ctx.Context)
	response.NewSuccessResponse("Success").FPrint(w)
}
