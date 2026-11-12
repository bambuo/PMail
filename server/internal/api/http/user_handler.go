package httpapi

import (
	"encoding/json"
	"io"
	"math"
	"net/http"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

// UserHandler 用户与账号接口：登录、口令、账号管理，经 UserService 访问持久化端口
type UserHandler struct {
	users *app.UserService
	mfa   *app.MFAService
}

// loginUserResponse 组装登录成功响应；api_key 为口令哈希，供按需铸造 5 秒时效 Token
func loginUserResponse(w http.ResponseWriter, user *models.User) {
	domains := config.Instance.Domains
	domains = array.Difference(domains, []string{config.Instance.Domain})
	domains = append([]string{config.Instance.Domain}, domains...)

	response.NewSuccessResponse(map[string]any{
		"account":  user.Account,
		"name":     user.Name,
		"is_admin": user.IsAdmin,
		"domains":  domains,
		"api_key":  user.Password,
	}).FPrint(w)
}

type httpLoginRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type httpModifyPasswordRequest struct {
	Password string `json:"password"`
}

type httpUserCreateRequest struct {
	Id       int    `json:"id"`
	Account  string `json:"account"`
	Username string `json:"username"`
	Password string `json:"password"`
	Disabled int    `json:"disabled"`
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

// Login 网页登录：校验口令，已绑定 MFA 时下发挑战
func (h *UserHandler) Login(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var reqData httpLoginRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	user, err := h.users.Authenticate(reqData.Account, reqData.Password)
	if err != nil {
		response.NewErrorResponse(response.ParamsError, app.TextGetText(ctx.Lang, "aperror"), "").FPrint(w)
		return
	}

	if app.MFAIsEnabled(user) {
		token, err := h.mfa.CreateChallenge(user.ID)
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
	userStr, _ := json.Marshal(*user)
	app.Sessions.Put(req.Context(), "user", string(userStr))
	loginUserResponse(w, user)
}

// Logout 退出登录，清除会话
func (h *UserHandler) Logout(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	app.Sessions.Clear(ctx.Context)
	response.NewSuccessResponse("Success").FPrint(w)
}

// ModifyPassword 修改当前登录用户口令
func (h *UserHandler) ModifyPassword(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.Errorf("%+v", err)
	}
	var reqData httpModifyPasswordRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.Errorf("%+v", err)
	}

	if reqData.Password != "" {
		if err := h.users.ModifyPassword(ctx.UserID, reqData.Password); err != nil {
			response.NewErrorResponse(response.ServerError, app.TextGetText(ctx.Lang, "unknowError"), "").FPrint(w)
			return
		}
	}

	response.NewSuccessResponse(app.TextGetText(ctx.Lang, "succ")).FPrint(w)
}

// Create 创建账号（管理员）
func (h *UserHandler) Create(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

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

	user, err := h.users.CreateAdministrator(reqData.Username, reqData.Account, reqData.Password)

	if err != nil {
		response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		return
	}

	response.NewSuccessResponse(user).FPrint(w)
}

// List 分页列出账号（管理员）
func (h *UserHandler) List(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
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

	users, totalNum, err := h.users.ListUsers(reqData.PageSize, offset)
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

// Info 返回当前登录用户信息
func (h *UserHandler) Info(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

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

// Edit 修改账号信息（管理员）
func (h *UserHandler) Edit(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

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
	var user *models.User
	if reqData.Id != 0 {
		user, err = h.users.FindUserByID(reqData.Id)
		if err != nil {
			log.Errorf("SQL Error: %+v", err)
		}
	} else {
		user, err = h.users.FindUserByName(reqData.Account)
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

	num, err := h.users.UpdateUser(user, reqData.Password)

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
