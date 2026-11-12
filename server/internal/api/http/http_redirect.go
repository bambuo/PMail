package httpapi

import (
	"net/http"

	"github.com/Jinnrry/pmail/internal/config"
)

func HTTPInterceptor(w http.ResponseWriter, r *http.Request) {
	URL := "https://" + config.Instance.WebDomain + r.URL.Path
	http.Redirect(w, r, URL, http.StatusMovedPermanently)
}
