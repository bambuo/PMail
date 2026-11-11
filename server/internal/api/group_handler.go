package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/array"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

func HTTPGetUserGroupList(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	defaultGroup := []*models.Group{
		{ID: models.INBOX, Name: app.TextGetText(ctx.Lang, "inbox"), FullPath: "/"},     // 收件箱
		{ID: models.Junk, Name: app.TextGetText(ctx.Lang, "junk"), FullPath: "/"},       //垃圾邮件
		{ID: models.Deleted, Name: app.TextGetText(ctx.Lang, "deleted"), FullPath: "/"}, //已删除
	}

	infos := app.GroupGetGroupList(ctx)

	response.NewSuccessResponse(append(defaultGroup, infos...)).FPrint(w)
}

func HTTPGetUserGroup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {

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

	retData = array.Merge(retData, app.GroupGetGroupInfoList(ctx))

	response.NewSuccessResponse(retData).FPrint(w)
}

type httpAddGroupRequest struct {
	Name     string `json:"name"`
	ParentId int    `json:"parent_id"`
}

func HTTPAddGroup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	var reqData *httpAddGroupRequest
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	newGroup, err := app.CreateGroup(ctx, reqData.Name, reqData.ParentId)

	if err != nil {
		response.NewErrorResponse(response.ServerError, "DBError", err.Error()).FPrint(w)
		return
	}

	response.NewSuccessResponse(newGroup.ID).FPrint(w)
}

type httpDelGroupRequest struct {
	Id int `json:"id"`
}

func HTTPDelGroup(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	var reqData *httpDelGroupRequest
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	succ, err := app.GroupDelGroup(ctx, reqData.Id)

	if err != nil {
		response.NewErrorResponse(response.ServerError, "DBError", err.Error()).FPrint(w)
		return
	}
	response.NewSuccessResponse(succ).FPrint(w)
}
