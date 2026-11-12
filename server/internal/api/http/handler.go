package httpapi

import (
	"net/http"

	"github.com/Jinnrry/pmail/pkg/context"
)

type HTTPHandlerFunc func(*context.Context, http.ResponseWriter, *http.Request)
