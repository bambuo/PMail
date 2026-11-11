package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
)

func EmailHTTPSend(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	data, err := io.ReadAll(req.Body)
	if err != nil {
		response.NewErrorResponse(response.ParamsError, "params error", err.Error()).FPrint(w)
		return
	}
	log.WithContext(ctx).Infof("发送邮件")
	var input app.SendEmailInput
	if err = json.Unmarshal(data, &input); err != nil {
		response.NewErrorResponse(response.ParamsError, "params error", err.Error()).FPrint(w)
		return
	}
	if err = app.SubmitEmail(ctx, input); err != nil {
		if failure, ok := err.(*app.UseCaseError); ok {
			response.NewErrorResponse(failure.Code, failure.Message, failure.Data).FPrint(w)
		} else {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		}
		return
	}
	response.NewSuccessResponse(app.TextGetText(ctx.Lang, "succ")).FPrint(w)
}
