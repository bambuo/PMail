package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	mfadto "github.com/Jinnrry/pmail/dto/mfa"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

type mfaSetupRequest struct {
	Account string `json:"account"`
}

type mfaConfirmRequest struct {
	Account string `json:"account"`
	Code    string `json:"code"`
}

type mfaDisableRequest struct {
	Password string `json:"password"`
	Code     string `json:"code"`
}

type mfaResetRequest struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type mfaLoginRequest struct {
	ChallengeToken string `json:"challengeToken"`
	Code           string `json:"code"`
}

// MFAHandler 用户 MFA 接口（含登录挑战），委托 MFA 用例函数
type MFAHandler struct{}

func (h *MFAHandler) Status(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		mfaFail(w, fmt.Errorf("method %s not allowed", req.Method))
		return
	}
	status, err := app.StatusMFA(ctx)
	if err != nil {
		mfaFail(w, err)
		return
	}
	response.NewSuccessResponse(status).FPrint(w)
}

func (h *MFAHandler) Setup(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		mfaFail(w, fmt.Errorf("method %s not allowed", req.Method))
		return
	}
	var body mfaSetupRequest
	if err := mfaDecodeOptional(req, &body); err != nil {
		mfaFail(w, err)
		return
	}
	if !mfaCanManage(ctx, body.Account) {
		mfaForbidden(w)
		return
	}
	data, err := app.SetupMFA(ctx, body.Account)
	if err != nil {
		mfaFail(w, err)
		return
	}
	log.WithContext(ctx).Infof("mfa action=setup account=%s target=%s", ctx.UserAccount, mfaTarget(body.Account, ctx.UserAccount))
	response.NewSuccessResponse(data).FPrint(w)
}

func (h *MFAHandler) Confirm(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		mfaFail(w, fmt.Errorf("method %s not allowed", req.Method))
		return
	}
	var body mfaConfirmRequest
	if err := mfaDecode(req, &body); err != nil || body.Code == "" {
		mfaFail(w, mfaErrorOrDefault(err, "code is required"))
		return
	}
	if !mfaCanManage(ctx, body.Account) {
		mfaForbidden(w)
		return
	}
	codes, err := app.ConfirmMFA(ctx, body.Account, body.Code)
	if err != nil {
		mfaFail(w, err)
		return
	}
	log.WithContext(ctx).Infof("mfa action=confirm account=%s target=%s", ctx.UserAccount, mfaTarget(body.Account, ctx.UserAccount))
	response.NewSuccessResponse(map[string]any{"recoveryCodes": codes}).FPrint(w)
}

func (h *MFAHandler) Disable(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		mfaFail(w, fmt.Errorf("method %s not allowed", req.Method))
		return
	}
	var body mfaDisableRequest
	if err := mfaDecode(req, &body); err != nil {
		mfaFail(w, err)
		return
	}
	if body.Password == "" || body.Code == "" {
		mfaFail(w, fmt.Errorf("password and code are required"))
		return
	}
	if err := app.DisableMFA(ctx, body.Password, body.Code); err != nil {
		mfaFail(w, err)
		return
	}
	log.WithContext(ctx).Infof("mfa action=disable account=%s", ctx.UserAccount)
	response.NewSuccessResponse("success").FPrint(w)
}

func (h *MFAHandler) Reset(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		mfaFail(w, fmt.Errorf("method %s not allowed", req.Method))
		return
	}
	if !ctx.IsAdmin {
		response.NewErrorResponse(response.NoAccessPrivileges, "No Access Privileges", "").FPrint(w)
		return
	}
	var body mfaResetRequest
	if err := mfaDecode(req, &body); err != nil || body.Account == "" || body.Password == "" {
		mfaFail(w, mfaErrorOrDefault(err, "account and password are required"))
		return
	}
	if err := app.ResetMFA(ctx, body.Account, body.Password); err != nil {
		mfaFail(w, err)
		return
	}
	log.WithContext(ctx).Infof("mfa action=reset account=%s target=%s", ctx.UserAccount, body.Account)
	response.NewSuccessResponse("success").FPrint(w)
}

func (h *MFAHandler) LoginMFA(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		mfaFail(w, fmt.Errorf("method %s not allowed", req.Method))
		return
	}
	var body mfaLoginRequest
	if err := mfaDecode(req, &body); err != nil {
		response.NewErrorResponse(response.ParamsError, app.TextGetText(ctx.Lang, "mfa_invalid"), "").FPrint(w)
		return
	}
	user, err := app.VerifyMFAChallenge(body.ChallengeToken, body.Code)
	if err != nil {
		log.WithContext(ctx).Warnf("mfa login failed: %v", err)
		response.NewErrorResponse(response.ParamsError, app.TextGetText(ctx.Lang, "mfa_invalid"), "").FPrint(w)
		return
	}
	userJSON, _ := json.Marshal(user)
	app.Sessions.Put(req.Context(), "user", string(userJSON))
	log.WithContext(ctx).Infof("mfa login success account=%s", user.Account)
	loginUserResponse(w, user)
}

func mfaDecode(req *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(req.Body, 1<<20))
	return dec.Decode(dst)
}

func mfaDecodeOptional(req *http.Request, dst any) error {
	if err := mfaDecode(req, dst); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func mfaCanManage(ctx *pcontext.Context, account string) bool {
	return account == "" || account == ctx.UserAccount || ctx.IsAdmin
}

func mfaForbidden(w http.ResponseWriter) {
	response.NewErrorResponse(response.NoAccessPrivileges, "No Access Privileges", "").FPrint(w)
}

func mfaTarget(account, self string) string {
	if account == "" {
		return self
	}
	return account
}

func mfaErrorOrDefault(err error, fallback string) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", fallback)
}

func mfaFail(w http.ResponseWriter, err error) {
	if err == nil {
		err = mfadto.ErrDisabled
	}
	response.NewErrorResponse(response.ParamsError, err.Error(), "").FPrint(w)
}
