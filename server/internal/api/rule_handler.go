package api

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
	"github.com/Jinnrry/pmail/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// RuleHandler 收信规则接口，绑定规则持久化端口
type RuleHandler struct {
	rules app.RulePersistence
}

func (h *RuleHandler) GetRule(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	res := app.RuleGetAllRules(ctx, ctx.UserID)
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

	err = h.save(ctx, data.Encode())
	if err != nil {
		response.NewErrorResponse(response.ServerError, "server error", err).FPrint(w)
		return
	}
	response.NewSuccessResponse("succ").FPrint(w)
}

func (h *RuleHandler) save(ctx *context.Context, p *models.Rule) error {

	if p.Id > 0 {
		_, err := h.rules.UpdateUserRule(ctx, p.Name, p.Value, p.Action, p.Params, p.Sort, p.Id, ctx.UserID)
		if err != nil {
			return errors.Wrap(err)
		}
		return nil
	} else {
		_, err := h.rules.InsertUserRule(ctx, p.Name, p.Value, ctx.UserID, p.Action, p.Params, p.Sort)
		if err != nil {
			return errors.Wrap(err)
		}
		return nil
	}

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

	_, err = h.rules.DeleteUserRule(ctx, data.Id, ctx.UserID)
	if err != nil {
		response.NewErrorResponse(response.ServerError, "unknown error", err).FPrint(w)
		return
	}

	response.NewSuccessResponse("succ").FPrint(w)
}
