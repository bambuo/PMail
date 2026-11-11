package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// httpServerSPAHandler serves static assets and falls back to index.html for
// history-mode routes. API and asset paths keep returning 404 when missing.
func httpServerSPAHandler(fe fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fe))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(fe, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") ||
			strings.HasPrefix(r.URL.Path, "/attachments/") ||
			strings.HasPrefix(r.URL.Path, "/.well-known/") ||
			strings.Contains(path, ".") {
			http.NotFound(w, r)
			return
		}
		index, err := fs.ReadFile(fe, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
}
