package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Jinnrry/pmail/dto/plugin"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/internal/config"
	pcontext "github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// PluginHandler 插件接口：列表、设置页与审批/启停等管理操作
type PluginHandler struct {
	plugins *app.PluginService
}

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

const maxPluginUploadBytes = 32 << 20

// GetList 返回已注册的插件钩子名称
func (h *PluginHandler) GetList(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	ret := []string{}
	for _, s := range h.plugins.HookNames() {
		ret = append(ret, s)
	}
	response.NewSuccessResponse(ret).FPrint(w)

}

// SettingsHtml 渲染指定插件的设置页
func (h *PluginHandler) SettingsHtml(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	args := strings.Split(req.RequestURI, "/")
	if len(args) < 4 {
		response.NewErrorResponse(response.ParamsError, "404", "").FPrint(w)
		return
	}

	pluginName := args[4]
	if plugin, ok := h.plugins.GetHook(pluginName); ok {
		dt, err := io.ReadAll(req.Body)
		if err != nil {
			response.NewErrorResponse(response.ParamsError, err.Error(), "").FPrint(w)
			return
		}
		html := plugin.SettingsHtml(ctx,
			strings.Join(args[4:], "/"),
			string(dt),
		)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		w.Write([]byte(html))
		return

	}
	response.NewErrorResponse(response.ParamsError, "404", "")
}

func (h *PluginHandler) ManageList(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	if !pluginManageRequireMethod(w, req, http.MethodGet) {
		return
	}
	if !pluginManageRequireAdmin(ctx, w) {
		return
	}
	list := h.plugins.List()
	if list == nil {
		list = []plugin.Info{}
	}
	enabled := config.Instance != nil && config.Instance.PluginWasmEnabled == 1
	response.NewSuccessResponse(pluginManageListResponse{Enabled: enabled, Plugins: list}).FPrint(w)
}

func (h *PluginHandler) ManageApprove(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
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
	if err := h.plugins.Approve(ctx, body.PluginID, body.Hooks, body.Capabilities); err != nil {
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

func (h *PluginHandler) ManageEnable(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "enable", h.plugins.Enable, string(plugin.StatusEnabled))
}

func (h *PluginHandler) ManageDisable(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "disable", h.plugins.Disable, string(plugin.StatusDisabled))
}

func (h *PluginHandler) ManageReject(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "reject", h.plugins.Reject, string(plugin.StatusRejected))
}

func (h *PluginHandler) ManageReload(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
	pluginManageSimpleAction(ctx, w, req, "reload", h.plugins.Reload, string(plugin.StatusEnabled))
}

func (h *PluginHandler) ManageUpload(ctx *pcontext.Context, w http.ResponseWriter, req *http.Request) {
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
	info, err := h.plugins.Install(data)
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
