package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/Jinnrry/pmail/config"
	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

type pluginManageListResponse struct {
	Enabled bool          `json:"enabled"`
	Plugins []plugin.Info `json:"plugins"`
}

type pluginApproveRequest struct {
	PluginID     string   `json:"pluginId"`
	Hooks        []string `json:"hooks"`
	Capabilities []string `json:"capabilities"`
}

type pluginIDRequest struct {
	PluginID string `json:"pluginId"`
}

type pluginStatusResponse struct {
	PluginID string `json:"pluginId"`
	Status   string `json:"status"`
}

type pluginApproveResponse struct {
	PluginID            string   `json:"pluginId"`
	Status              string   `json:"status"`
	GrantedHooks        []string `json:"grantedHooks"`
	GrantedCapabilities []string `json:"grantedCapabilities"`
}

func HTTPPluginManageList(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if !pluginManageRequireMethod(w, req, http.MethodGet) {
		return
	}
	if !pluginManageRequireAdmin(ctx, w) {
		return
	}
	list := app.GetPluginManageList()
	if list == nil {
		list = []plugin.Info{}
	}
	enabled := config.Instance != nil && config.Instance.PluginWasmEnabled == 1
	response.NewSuccessResponse(pluginManageListResponse{Enabled: enabled, Plugins: list}).FPrint(w)
}

func HTTPPluginManageApprove(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if !pluginManageRequireMethod(w, req, http.MethodPost) || !pluginManageRequireAdmin(ctx, w) {
		return
	}
	var body pluginApproveRequest
	if err := pluginManageDecode(req, &body); err != nil {
		pluginManageFail(w, response.ParamsError, err)
		return
	}
	if body.PluginID == "" {
		pluginManageFail(w, response.ParamsError, errors.New("pluginId is required"))
		return
	}
	if err := app.ApprovePlugin(ctx, body.PluginID, body.Hooks, body.Capabilities); err != nil {
		pluginManageError(w, err)
		return
	}
	log.WithContext(ctx).Infof("plugin_manage action=approve plugin=%s hooks=%v capabilities=%v operator=%s",
		body.PluginID, body.Hooks, body.Capabilities, ctx.UserAccount)
	response.NewSuccessResponse(pluginApproveResponse{
		PluginID:            body.PluginID,
		Status:              string(plugin.StatusApproved),
		GrantedHooks:        body.Hooks,
		GrantedCapabilities: body.Capabilities,
	}).FPrint(w)
}

func HTTPPluginManageEnable(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "enable", app.EnablePlugin, string(plugin.StatusEnabled))
}

func HTTPPluginManageDisable(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "disable", app.DisablePlugin, string(plugin.StatusDisabled))
}

func HTTPPluginManageReject(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "reject", app.RejectPlugin, string(plugin.StatusRejected))
}

func HTTPPluginManageReload(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "reload", app.ReloadPlugin, string(plugin.StatusEnabled))
}

const maxPluginUploadBytes = 32 << 20

func HTTPPluginManageUpload(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if !pluginManageRequireMethod(w, req, http.MethodPost) || !pluginManageRequireAdmin(ctx, w) {
		return
	}
	req.Body = http.MaxBytesReader(w, req.Body, maxPluginUploadBytes+1<<20)
	if err := req.ParseMultipartForm(maxPluginUploadBytes); err != nil {
		pluginManageFail(w, response.ParamsError, fmt.Errorf("parse upload: %w", err))
		return
	}
	file, _, err := req.FormFile("file")
	if err != nil {
		pluginManageFail(w, response.ParamsError, errors.New("file is required"))
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPluginUploadBytes+1))
	if err != nil {
		pluginManageFail(w, response.ParamsError, fmt.Errorf("read upload: %w", err))
		return
	}
	if len(data) == 0 || len(data) > maxPluginUploadBytes {
		pluginManageFail(w, response.ParamsError, fmt.Errorf("file must be between 1 byte and %d bytes", maxPluginUploadBytes))
		return
	}
	info, err := app.InstallPlugin(data)
	if err != nil {
		pluginManageFail(w, response.ParamsError, err)
		return
	}
	log.WithContext(ctx).Infof("plugin_manage action=upload plugin=%s operator=%s", info.ID, ctx.UserAccount)
	response.NewSuccessResponse(info).FPrint(w)
}

func pluginManageSimpleAction(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request, actionName string, action func(*pcontext.Context, string) error, status string) {
	if !pluginManageRequireMethod(w, req, http.MethodPost) || !pluginManageRequireAdmin(ctx, w) {
		return
	}
	var body pluginIDRequest
	if err := pluginManageDecode(req, &body); err != nil {
		pluginManageFail(w, response.ParamsError, err)
		return
	}
	if body.PluginID == "" {
		pluginManageFail(w, response.ParamsError, errors.New("pluginId is required"))
		return
	}
	if err := action(ctx, body.PluginID); err != nil {
		pluginManageError(w, err)
		return
	}
	log.WithContext(ctx).Infof("plugin_manage action=%s plugin=%s operator=%s status=%s",
		actionName, body.PluginID, ctx.UserAccount, status)
	response.NewSuccessResponse(pluginStatusResponse{PluginID: body.PluginID, Status: status}).FPrint(w)
}

func pluginManageRequireAdmin(ctx *pcontext.Context, w http.ResponseWriter) bool {
	if ctx == nil || !ctx.IsAdmin {
		response.NewErrorResponse(response.NoAccessPrivileges, "no access privileges", nil).FPrint(w)
		return false
	}
	return true
}

func pluginManageRequireMethod(w http.ResponseWriter, req *http.Request, method string) bool {
	if req.Method != method {
		pluginManageFail(w, response.ParamsError, fmt.Errorf("method %s not allowed", req.Method))
		return false
	}
	return true
}

func pluginManageDecode(req *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(req.Body, 1<<20))
	return dec.Decode(dst)
}

func pluginManageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, plugin.ErrNotFound),
		errors.Is(err, plugin.ErrInvalidState),
		errors.Is(err, plugin.ErrInvalidGrant),
		errors.Is(err, plugin.ErrNameConflict),
		errors.Is(err, plugin.ErrDisabled):
		pluginManageFail(w, response.ParamsError, err)
	default:
		pluginManageFail(w, response.ServerError, err)
	}
}

func pluginManageFail(w http.ResponseWriter, errorNo int, err error) {
	response.NewErrorResponse(errorNo, err.Error(), nil).FPrint(w)
}
