package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// web/ is produced by the frontend build (vite outDir).
//
//go:embed all:web
var webFS embed.FS

func (a *App) staticHandler() http.Handler {
	sub, _ := fs.Sub(webFS, "web")
	files := http.FileServer(http.FS(sub))
	serveHTML := func(w http.ResponseWriter, r *http.Request, name string) {
		b, err := fs.ReadFile(sub, name)
		if err != nil {
			http.Error(w, "frontend not built", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Write(b)
	}
	return gzipped(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/studio" || strings.HasPrefix(p, "/studio/"):
			serveHTML(w, r, "studio.html")
			return
		case p == "/console" || strings.HasPrefix(p, "/console/"):
			serveHTML(w, r, "console.html")
			return
		case strings.HasPrefix(p, "/assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case p == "/sw.js":
			w.Header().Set("Cache-Control", "no-cache")
		}
		if p == "/" || path.Ext(p) == "" {
			serveHTML(w, r, "index.html")
			return
		}
		if _, err := fs.Stat(sub, strings.TrimPrefix(p, "/")); err != nil {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	}))
}
