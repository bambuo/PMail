package api

import (
	"encoding/json"
	"io"
	"math"
	"net/http"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	"github.com/Jinnrry/pmail/pkg/password"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

type httpUserCreateRequest struct {
	Id       int    `json:"id"`
	Account  string `json:"account"`
	Username string `json:"username"`
	Password string `json:"password"`
	Disabled int    `json:"disabled"`
}

func HTTPCreateUser(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

	if !ctx.IsAdmin {
		response.NewErrorResponse(response.NoAccessPrivileges, "No Access Privileges", "").FPrint(w)
		return
	}

	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var reqData httpUserCreateRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	if reqData.Username == "" || reqData.Password == "" || reqData.Account == "" {
		response.NewErrorResponse(response.ParamsError, "Params Error", "").FPrint(w)
		return
	}

	var user models.User
	user.Name = reqData.Username
	user.Password = password.Encode(reqData.Password)
	user.Account = reqData.Account

	_, err = app.Persistence.InsertAdministrator(&user)

	if err != nil {
		response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		return
	}

	response.NewSuccessResponse(user).FPrint(w)
}

type httpUserListRequest struct {
	CurrentPage int `json:"current_page"`
	PageSize    int `json:"page_size"`
}

type httpUserListItem struct {
	ID                        int    `json:"ID"`
	Account                   string `json:"Account"`
	Name                      string `json:"Name"`
	Disabled                  int    `json:"Disabled"`
	MfaEnabled                bool   `json:"MfaEnabled"`
	MfaRecoveryCodesRemaining int    `json:"MfaRecoveryCodesRemaining"`
}

func HTTPUserList(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	if !ctx.IsAdmin {
		response.NewErrorResponse(response.NoAccessPrivileges, "No Access Privileges", "").FPrint(w)
		return
	}

	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var reqData httpUserListRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	offset := 0
	if reqData.CurrentPage >= 1 {
		offset = (reqData.CurrentPage - 1) * reqData.PageSize
	}

	if reqData.PageSize == 0 {
		reqData.PageSize = 15
	}

	var users []models.User

	totalNum, err := app.Persistence.ListAccounts(reqData.PageSize, offset, &users)
	if err != nil {
		log.Errorf("%+v", err)
	}

	list := make([]httpUserListItem, 0, len(users))
	for _, user := range users {
		item := httpUserListItem{
			ID:         user.ID,
			Account:    user.Account,
			Name:       user.Name,
			Disabled:   user.Disabled,
			MfaEnabled: user.MfaEnabled == 1,
		}
		if user.MfaRecoveryCodes != "" {
			var hashes []string
			if err := json.Unmarshal([]byte(user.MfaRecoveryCodes), &hashes); err == nil {
				item.MfaRecoveryCodesRemaining = len(hashes)
			}
		}
		list = append(list, item)
	}

	response.NewSuccessResponse(map[string]any{
		"current_page": reqData.CurrentPage,
		"total_page":   cast.ToInt(math.Ceil(cast.ToFloat64(totalNum) / cast.ToFloat64(reqData.PageSize))),
		"list":         list,
	}).FPrint(w)

}

func HTTPInfo(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

	domains := config.Instance.Domains
	domains = array.Difference(domains, []string{config.Instance.Domain})
	domains = append([]string{config.Instance.Domain}, domains...)

	response.NewSuccessResponse(map[string]any{
		"account":  ctx.UserAccount,
		"name":     ctx.UserName,
		"is_admin": ctx.IsAdmin,
		"domains":  domains,
	}).FPrint(w)
}

func HTTPEditUser(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

	if !ctx.IsAdmin {
		response.NewErrorResponse(response.NoAccessPrivileges, "No Access Privileges", "").FPrint(w)
		return
	}

	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var reqData httpUserCreateRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	if reqData.Id == 0 && reqData.Account == "" {
		response.NewErrorResponse(response.ParamsError, "Params Error", "").FPrint(w)
		return
	}
	var user models.User
	if reqData.Id != 0 {
		_, err = app.Persistence.FindAccountByID(reqData.Id, &user)
		if err != nil {
			log.Errorf("SQL Error: %+v", err)
		}
	} else {
		_, err = app.Persistence.FindAccountByName(reqData.Account, &user)
		if err != nil {
			log.Errorf("SQL Error: %+v", err)
		}
	}
	if user.ID == 0 {
		response.NewErrorResponse(response.ParamsError, "User not found", "").FPrint(w)
		return
	}
	if reqData.Username != "" && reqData.Username != user.Name {
		user.Name = reqData.Username
	}

	if reqData.Disabled != user.Disabled {
		user.Disabled = reqData.Disabled
	}
	if reqData.Password != "" {
		user.Password = password.Encode(reqData.Password)
	}

	num, err := app.Persistence.UpdateAccount(user.ID, &user)

	if err != nil {
		response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		return
	}
	if num == 0 {
		response.NewErrorResponse(response.ServerError, "No Data Update", "").FPrint(w)
		return
	}

	response.NewSuccessResponse(user).FPrint(w)
}
