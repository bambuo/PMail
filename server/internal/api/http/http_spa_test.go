package httpapi

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHTTPServerSPAHandler(t *testing.T) {
	files := fstest.MapFS{
		"index.html":    {Data: []byte("<html>app</html>")},
		"assets/app.js": {Data: []byte("console.log('app')")},
	}
	handler := httpServerSPAHandler(fs.FS(files))

	tests := []struct {
		path       string
		wantStatus int
		wantBody   string
	}{
		{"/", http.StatusOK, "<html>app</html>"},
		{"/detail/1", http.StatusOK, "<html>app</html>"},
		{"/setup", http.StatusOK, "<html>app</html>"},
		{"/assets/app.js", http.StatusOK, "console.log('app')"},
		{"/assets/missing.js", http.StatusNotFound, ""},
		{"/api/unknown", http.StatusNotFound, ""},
		{"/attachments/missing", http.StatusNotFound, ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Fatalf("body = %q, want contains %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
