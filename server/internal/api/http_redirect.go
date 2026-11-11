package api

import (
	"net/http"

	"github.com/Jinnrry/pmail/config"
)

func HTTPInterceptor(w http.ResponseWriter, r *http.Request) {
	URL := "https://" + config.Instance.WebDomain + r.URL.Path
	http.Redirect(w, r, URL, http.StatusMovedPermanently)
}
