package frontend_test

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/6space7/porter/internal/frontend"
)

func TestHandlerServesStaticFilesAndFallsBackToIndex(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":         {Data: []byte(`<div id="app"></div><script src="/assets/app.js"></script>`)},
		"assets/app.js":      {Data: []byte(`console.log("porter")`)},
		"assets/app.css":     {Data: []byte(`body{margin:0}`)},
		"assets/nested/info": {Data: []byte(`not a directory listing`)},
	}
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "api route", http.StatusTeapot)
	})
	handler := frontend.NewHandler(api, assets)

	assertBody(t, handler, http.MethodGet, "/", http.StatusOK, `<div id="app"></div><script src="/assets/app.js"></script>`)
	assertBody(t, handler, http.MethodGet, "/assets/app.js", http.StatusOK, `console.log("porter")`)
	assertBody(t, handler, http.MethodGet, "/apps/app_1", http.StatusOK, `<div id="app"></div><script src="/assets/app.js"></script>`)
	assertBody(t, handler, http.MethodGet, "/api/v1/apps", http.StatusTeapot, "api route\n")
	assertBody(t, handler, http.MethodGet, "/health", http.StatusTeapot, "api route\n")
}

func TestHandlerReturnsNotFoundWhenIndexIsMissing(t *testing.T) {
	handler := frontend.NewHandler(http.NotFoundHandler(), fstest.MapFS{})

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestHandlerDoesNotServeDirectories(t *testing.T) {
	assets := fstest.MapFS{
		"index.html": {Data: []byte("index")},
		"assets":     {Mode: fs.ModeDir},
	}
	handler := frontend.NewHandler(http.NotFoundHandler(), assets)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/assets", nil))

	if rr.Code != http.StatusOK || rr.Body.String() != "index" {
		t.Fatalf("directory fallback status/body = %d/%q, want index fallback", rr.Code, rr.Body.String())
	}
}

func assertBody(t *testing.T, handler http.Handler, method, path string, wantStatus int, wantBody string) {
	t.Helper()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(method, path, nil))

	if rr.Code != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, path, rr.Code, wantStatus, rr.Body.String())
	}
	if rr.Body.String() != wantBody {
		t.Fatalf("%s %s body = %q, want %q", method, path, rr.Body.String(), wantBody)
	}
}
