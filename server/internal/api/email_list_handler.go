package api

import (
	"encoding/json"
	"io"
	"math"
	"net/http"

	"github.com/Jinnrry/pmail/dto"
	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/dto/response"
	"github.com/Jinnrry/pmail/internal/app"
	"github.com/Jinnrry/pmail/pkg/context"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cast"
)

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

func EmailHTTPEmailList(ctx *context.Context, w http.ResponseWriter, req *http.Request) {
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

	emailList, total := app.ListEmails(ctx, tagInfo, retData.Keyword, false, offset, retData.PageSize)

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
