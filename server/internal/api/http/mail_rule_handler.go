package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/address"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

// GroupHandler 邮件分组接口，委托分组用例服务
type GroupHandler struct {
	groups *app.GroupService
}

func (h *GroupHandler) GetUserGroupList(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	defaultGroup := []*models.Group{
		{ID: models.INBOX, Name: app.TextGetText(ctx.Lang, "inbox"), FullPath: "/"},     // 收件箱
		{ID: models.Junk, Name: app.TextGetText(ctx.Lang, "junk"), FullPath: "/"},       //垃圾邮件
		{ID: models.Deleted, Name: app.TextGetText(ctx.Lang, "deleted"), FullPath: "/"}, //已删除
	}

	infos := h.groups.List(ctx)

	response.NewSuccessResponse(append(defaultGroup, infos...)).FPrint(w)
}

func (h *GroupHandler) GetUserGroup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

	retData := []*app.GroupItem{
		{
			Label: app.TextGetText(ctx.Lang, "inbox"),
			Tag:   dto.SearchTag{Type: 0, Status: -1, GroupId: 0}.ToString(),
		},
		{
			Label: app.TextGetText(ctx.Lang, "outbox"),
			Tag:   dto.SearchTag{Type: 1, Status: -1}.ToString(),
		},
		{
			Label: app.TextGetText(ctx.Lang, "sketch"),
			Tag:   dto.SearchTag{Type: 0, Status: 4}.ToString(),
		},
		{
			Label: app.TextGetText(ctx.Lang, "junk"),
			Tag:   dto.SearchTag{Type: -1, Status: 5}.ToString(),
		},
		{
			Label: app.TextGetText(ctx.Lang, "deleted"),
			Tag:   dto.SearchTag{Type: -1, Status: 3}.ToString(),
		},
	}

	retData = array.Merge(retData, h.groups.InfoList(ctx))

	response.NewSuccessResponse(retData).FPrint(w)
}

type httpAddGroupRequest struct {
	Name     string `json:"name"`
	ParentId int    `json:"parent_id"`
}

func (h *GroupHandler) AddGroup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	var reqData *httpAddGroupRequest
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	newGroup, err := h.groups.Create(ctx, reqData.Name, reqData.ParentId)

	if err != nil {
		response.NewErrorResponse(response.ServerError, "DBError", err.Error()).FPrint(w)
		return
	}

	response.NewSuccessResponse(newGroup.ID).FPrint(w)
}

type httpDelGroupRequest struct {
	Id int `json:"id"`
}

func (h *GroupHandler) DelGroup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	var reqData *httpDelGroupRequest
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	succ, err := h.groups.Delete(ctx, reqData.Id)

	if err != nil {
		response.NewErrorResponse(response.ServerError, "DBError", err.Error()).FPrint(w)
		return
	}
	response.NewSuccessResponse(succ).FPrint(w)
}

// RuleHandler 收信规则接口，绑定规则用例服务
type RuleHandler struct {
	rules *app.RuleService
}

func (h *RuleHandler) GetRule(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	res := h.rules.GetAll(ctx, ctx.UserID)
	response.NewSuccessResponse(res).FPrint(w)
}

func (h *RuleHandler) UpsertRule(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

	requestBody, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("ReadError:%v", err)
		return
	}

	var data *dto.Rule
	err = json.Unmarshal(requestBody, &data)
	if err != nil {
		response.NewErrorResponse(response.ParamsError, "params error", err).FPrint(w)
		return
	}

	if data.Action == dto.FORWARD && !address.IsValidEmailAddress(data.Params) {

		response.NewErrorResponse(response.ParamsError, "ParamsError error", app.TextGetText(ctx.Lang, "invalid_email_address")).FPrint(w)
		return
	}

	for _, r := range data.Rules {
		if !array.InArray(r.Field, []string{"From", "Subject", "To", "Cc", "Text", "Html", "Content"}) {
			response.NewErrorResponse(response.ParamsError, "ParamsError error", "params error! Rule Field Error!").FPrint(w)
			return
		}
	}

	err = h.rules.Upsert(ctx, ctx.UserID, data.Encode())
	if err != nil {
		response.NewErrorResponse(response.ServerError, "server error", err).FPrint(w)
		return
	}
	response.NewSuccessResponse("succ").FPrint(w)
}

type httpDelRuleReq struct {
	Id int `json:"id"`
}

func (h *RuleHandler) DelRule(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	requestBody, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("ReadError:%v", err)
		return
	}

	var data httpDelRuleReq
	err = json.Unmarshal(requestBody, &data)
	if err != nil {
		response.NewErrorResponse(response.ParamsError, "params error", err).FPrint(w)
		return
	}

	if data.Id <= 0 {
		response.NewErrorResponse(response.ParamsError, "params error", "id is empty").FPrint(w)
		return
	}

	err = h.rules.Delete(ctx, data.Id, ctx.UserID)
	if err != nil {
		response.NewErrorResponse(response.ServerError, "unknown error", err).FPrint(w)
		return
	}

	response.NewSuccessResponse("succ").FPrint(w)
}
