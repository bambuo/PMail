package api

import (
	"net/http"

	"github.com/Jinnrry/pmail/dto/response"
)

func HTTPPing(w http.ResponseWriter, req *http.Request) {
	response.NewSuccessResponse("pong").FPrint(w)
}
