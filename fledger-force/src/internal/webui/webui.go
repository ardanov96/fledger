// Package webui embeds the static PWA files so the binary is self-contained.
package webui

import (
	"embed"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

//go:embed all:files
var files embed.FS

func mustSub() fs.FS {
	sub, err := fs.Sub(files, "files")
	if err != nil {
		panic("webui: missing files directory: " + err.Error())
	}
	return sub
}

// Handler returns the embedded PWA handler.
func Handler() http.Handler {
	subfs := mustSub()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := strings.TrimPrefix(r.URL.Path, "/")
		if upath == "" {
			upath = "index.html"
		}
		if _, err := fs.Stat(subfs, upath); err != nil {
			upath = "index.html"
		}
		f, err := subfs.Open(upath)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		defer f.Close()

		if upath == "sw.js" {
			w.Header().Set("Service-Worker-Allowed", "/")
		}
		w.Header().Set("Content-Type", mimeByExt(path.Ext(upath)))
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		_, _ = io.Copy(w, f)
	})
}

func mimeByExt(ext string) string {
	switch ext {
	case ".json", ".webmanifest":
		return "application/manifest+json"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".png":
		return "image/png"
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}