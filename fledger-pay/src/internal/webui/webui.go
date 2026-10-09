// Package webui embeds the static web portal files so the binary is fully
// self-contained.
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

// sub returns the http.FileSystem rooted at the embedded `files/` dir.
func sub() fs.FS { return mustSub() }

func mustSub() fs.FS {
	sub, err := fs.Sub(files, "files")
	if err != nil {
		panic("webui: missing files directory: " + err.Error())
	}
	return sub
}

// MIMEByExt returns the Content-Type for a file extension, defaulting to
// application/octet-stream.
func mimeByExt(ext string) string {
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

// Handler returns an http.Handler serving the embedded portal. The handler
// resolves "/" to "index.html" and serves other paths verbatim; unknown
// paths fall back to index.html.
func Handler() http.Handler {
	subfs := sub()
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
		w.Header().Set("Content-Type", mimeByExt(path.Ext(upath)))
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = io.Copy(w, f)
	})
}