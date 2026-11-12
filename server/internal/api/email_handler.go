package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

// EmailHandler 邮件列表与操作接口，委托邮件读写用例服务
type EmailHandler struct {
	queries *app.EmailQueryService
	emails  *app.EmailService
}

type emailHTTPEmailListResponse struct {
	CurrentPage int                  `json:"current_page"`
	TotalPage   int                  `json:"total_page"`
	Total       int64                `json:"total"`
	List        []*emailHTTPEmilItem `json:"list"`
}

type emailHTTPEmilItem struct {
	ID        int             `json:"id"`
	Title     string          `json:"title"`
	Desc      string          `json:"desc"`
	Datetime  string          `json:"datetime"`
	IsRead    bool            `json:"is_read"`
	Sender    EmailHTTPUser   `json:"sender"`
	To        []EmailHTTPUser `json:"to"`
	Dangerous bool            `json:"dangerous"`
	Error     string          `json:"error"`
}

type EmailHTTPUser struct {
	Name         string `json:"Name"`
	EmailAddress string `json:"EmailAddress"`
}

type emailHTTPEmailRequest struct {
	Keyword     string `json:"keyword"`
	Tag         string `json:"tag"`
	CurrentPage int    `json:"current_page"`
	PageSize    int    `json:"page_size"`
}

func (h *EmailHandler) List(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	var lst []*emailHTTPEmilItem
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	var retData emailHTTPEmailRequest
	err = json.Unmarshal(reqBytes, &retData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	offset := 0
	if retData.CurrentPage >= 1 {
		offset = (retData.CurrentPage - 1) * retData.PageSize
	}

	if retData.PageSize == 0 {
		retData.PageSize = 15
	}

	var tagInfo dto.SearchTag = dto.SearchTag{
		Type:    -1,
		Status:  -1,
		GroupId: -1,
	}
	_ = json.Unmarshal([]byte(retData.Tag), &tagInfo)

	emailList, total := h.queries.List(ctx, tagInfo, retData.Keyword, false, offset, retData.PageSize)

	for _, email := range emailList {
		var sender EmailHTTPUser
		_ = json.Unmarshal([]byte(email.Sender), &sender)

		if sender.EmailAddress == "" {
			sender.EmailAddress = email.FromAddress
			sender.Name = email.FromName
		}

		var tos []EmailHTTPUser
		_ = json.Unmarshal([]byte(email.To), &tos)
		authentication := parsemail.NewEmailAuthentication(email.SPFCheck == 1, email.DKIMCheck == 1)

		lst = append(lst, &emailHTTPEmilItem{
			ID:        email.Id,
			Title:     email.Subject,
			Desc:      email.Text.String,
			Datetime:  email.SendDate.Format("2006-01-02 15:04:05"),
			IsRead:    email.IsRead == 1,
			Sender:    sender,
			To:        tos,
			Dangerous: authentication.Dangerous,
			Error:     email.Error.String,
		})
	}

	ret := emailHTTPEmailListResponse{
		CurrentPage: retData.CurrentPage,
		TotalPage:   cast.ToInt(math.Ceil(cast.ToFloat64(total) / cast.ToFloat64(retData.PageSize))),
		Total:       total,
		List:        lst,
	}
	response.NewSuccessResponse(ret).FPrint(w)
}

type emailHTTPEmailDetailRequest struct {
	ID int `json:"id"`
}

func (h *EmailHandler) Detail(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	var retData emailHTTPEmailDetailRequest
	err = json.Unmarshal(reqBytes, &retData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	if retData.ID <= 0 {
		response.NewErrorResponse(response.ParamsError, "ID错误", "").FPrint(w)
		return
	}

	email, err := h.queries.Detail(ctx, retData.ID, true)
	if err != nil {
		response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		return
	}

	response.NewSuccessResponse(email).FPrint(w)

}

func (h *EmailHandler) Send(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
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
	if err = h.emails.Submit(ctx, input); err != nil {
		if failure, ok := err.(*app.UseCaseError); ok {
			response.NewErrorResponse(failure.Code, failure.Message, failure.Data).FPrint(w)
		} else {
			response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		}
		return
	}
	response.NewSuccessResponse(app.TextGetText(ctx.Lang, "succ")).FPrint(w)
}

type emailHTTPMarkReadRequest struct {
	IDs []int `json:"ids"`
}

func (h *EmailHandler) MarkRead(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	var reqData emailHTTPMarkReadRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	if len(reqData.IDs) <= 0 {
		response.NewErrorResponse(response.ParamsError, "ID错误", "").FPrint(w)
		return
	}

	for _, id := range reqData.IDs {
		h.queries.Detail(ctx, id, true)
	}

	if err != nil {
		response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		return
	}
	response.NewSuccessResponse("success").FPrint(w)

}

type emailHTTPEmailDeleteRequest struct {
	IDs       []int `json:"ids"`
	ForcedDel bool  `json:"forcedDel"`
}

func (h *EmailHandler) Delete(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	var reqData emailHTTPEmailDeleteRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	if len(reqData.IDs) <= 0 {
		response.NewErrorResponse(response.ParamsError, "ID错误", "").FPrint(w)
		return
	}

	err = h.emails.Delete(ctx, reqData.IDs, reqData.ForcedDel)
	if err != nil {
		response.NewErrorResponse(response.ServerError, err.Error(), "").FPrint(w)
		return
	}
	response.NewSuccessResponse("success").FPrint(w)

}

type emailHTTPMoveRequest struct {
	GroupId   int    `json:"group_id"`
	GroupName string `json:"group_name"`
	IDs       []int  `json:"ids"`
}

func (h *EmailHandler) Move(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	reqBytes, err := io.ReadAll(req.Body)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}
	var reqData emailHTTPMoveRequest
	err = json.Unmarshal(reqBytes, &reqData)
	if err != nil {
		log.WithContext(ctx).Errorf("%+v", err)
	}

	if len(reqData.IDs) <= 0 {
		response.NewErrorResponse(response.ParamsError, "ID错误", "").FPrint(w)
		return
	}

	if name, ok := models.GroupCodeToName[reqData.GroupId]; ok {
		err := h.emails.MoveToDefaultBox(ctx, reqData.IDs, name)
		if err != nil {
			response.NewErrorResponse(response.ServerError, "Error", err.Error()).FPrint(w)
			return
		}
	} else if !h.emails.MoveToGroup(ctx, reqData.IDs, reqData.GroupId) {
		response.NewErrorResponse(response.ServerError, "Error", "").FPrint(w)
		return
	}
	response.NewSuccessResponse("success").FPrint(w)

}

func (h *EmailHandler) GetAttachments(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	urlInfos := strings.Split(req.RequestURI, "/")
	if len(urlInfos) != 4 {
		response.NewErrorResponse(response.ParamsError, "", "").FPrint(w)
		return
	}
	emailId := cast.ToInt(urlInfos[2])
	cid := urlInfos[3]

	contentType, content := h.queries.Attachments(ctx, emailId, cid)

	if len(content) == 0 {
		response.NewErrorResponse(response.ParamsError, "", "").FPrint(w)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func (h *EmailHandler) Download(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
	urlInfos := strings.Split(req.RequestURI, "/")
	if len(urlInfos) != 5 {
		response.NewErrorResponse(response.ParamsError, "", "").FPrint(w)
		return
	}
	emailId := cast.ToInt(urlInfos[3])
	index := cast.ToInt(urlInfos[4])

	fileName, content := h.queries.AttachmentsByIndex(ctx, emailId, index)

	if len(content) == 0 {
		response.NewErrorResponse(response.ParamsError, "", "").FPrint(w)
		return
	}
	w.Header().Set("ContentType", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", fileName))
	w.Write(content)
}
