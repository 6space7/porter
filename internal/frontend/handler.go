package frontend

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

func NewHandler(api http.Handler, assets fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if routeToAPI(r.URL.Path) {
			api.ServeHTTP(w, r)
			return
		}

		requested := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if requested == "." || requested == "" {
			serveAsset(w, r, assets, "index.html")
			return
		}
		if fileExists(assets, requested) {
			serveAsset(w, r, assets, requested)
			return
		}
		serveAsset(w, r, assets, "index.html")
	})
}

func routeToAPI(requestPath string) bool {
	return requestPath == "/health" || strings.HasPrefix(requestPath, "/api/")
}

func serveAsset(w http.ResponseWriter, r *http.Request, assets fs.FS, name string) {
	if !fileExists(assets, name) {
		http.NotFound(w, r)
		return
	}
	http.ServeFileFS(w, r, assets, name)
}

func fileExists(assets fs.FS, name string) bool {
	file, err := fs.Stat(assets, name)
	return err == nil && !file.IsDir()
}
