package server

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// The review page, built from ui/ with `npm run build`.
//
//go:embed ui
var uiFiles embed.FS

func uiHandler() http.Handler {
	root, err := fs.Sub(uiFiles, "ui")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Everything the page loads comes from this server.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(root, name); err != nil {
			r.URL.Path = "/" // unknown paths get the page itself
		}
		files.ServeHTTP(w, r)
	})
}
