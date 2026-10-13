package main

import (
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
)

//go:embed index.html
var indexHTML []byte

//go:embed hub.html
var hubHTML []byte

//go:embed demo.html
var demoHTML []byte

//go:embed simulator.html
var simulatorHTML []byte

//go:embed assets/*
var assetsFS embed.FS

func proxyHandler(targetURL string, prefix string) http.Handler {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("invalid target url %s: %v", targetURL, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
		if prefix != "" && strings.HasPrefix(req.URL.Path, prefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, strings.TrimSuffix(prefix, "/"))
			if !strings.HasPrefix(req.URL.Path, "/") {
				req.URL.Path = "/" + req.URL.Path
			}
		}
	}
	return proxy
}

// Gzip Writer Pool for minimal allocation under load
var gzipWriterPool = sync.Pool{
	New: func() interface{} {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return gz
	},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if code == http.StatusNotModified || code == http.StatusNoContent {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Add("Vary", "Accept-Encoding")
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		w.writer.Flush()
		flusher.Flush()
	}
}

// gzipMiddleware automatically compresses HTML, JSON, CSS, and JS responses
func gzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass gzip if client doesn't support it
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// Bypass range requests (essential for smooth video/audio streaming)
		if r.Header.Get("Range") != "" {
			next.ServeHTTP(w, r)
			return
		}

		// Bypass already compressed media files
		path := strings.ToLower(r.URL.Path)
		if strings.HasSuffix(path, ".mp4") ||
			strings.HasSuffix(path, ".webm") ||
			strings.HasSuffix(path, ".webp") ||
			strings.HasSuffix(path, ".jpg") ||
			strings.HasSuffix(path, ".jpeg") ||
			strings.HasSuffix(path, ".png") ||
			strings.HasSuffix(path, ".gif") ||
			strings.HasSuffix(path, ".woff2") ||
			strings.HasSuffix(path, ".zip") ||
			strings.HasSuffix(path, ".gz") {
			next.ServeHTTP(w, r)
			return
		}

		// Bypass reverse proxy microservices
		if strings.HasPrefix(path, "/core/") ||
			strings.HasPrefix(path, "/fleet/") ||
			strings.HasPrefix(path, "/pay/") ||
			strings.HasPrefix(path, "/force/") ||
			strings.HasPrefix(path, "/order/") ||
			strings.HasPrefix(path, "/dunning/") {
			next.ServeHTTP(w, r)
			return
		}

		gz := gzipWriterPool.Get().(*gzip.Writer)
		defer gzipWriterPool.Put(gz)

		gz.Reset(w)
		defer gz.Close()

		gzw := &gzipResponseWriter{
			ResponseWriter: w,
			writer:         gz,
		}

		next.ServeHTTP(gzw, r)
	})
}

func main() {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8000"
	}

	mux := http.NewServeMux()

	// Central Gateway Healthcheck
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"alive","gateway":"fledger-gateway","port":%s}`, port)
	})

	// Reverse Proxies
	mux.Handle("/core/", proxyHandler("http://127.0.0.1:8081", "/core"))
	mux.Handle("/fleet/", proxyHandler("http://127.0.0.1:8082", "/fleet"))
	mux.Handle("/pay/", proxyHandler("http://127.0.0.1:8083", "/pay"))
	mux.Handle("/force/", proxyHandler("http://127.0.0.1:8084", "/force"))
	mux.Handle("/order/", proxyHandler("http://127.0.0.1:8085", "/order"))
	mux.Handle("/dunning/", proxyHandler("http://127.0.0.1:8086", "/dunning"))

	// Developer & QA Operations Hub
	mux.HandleFunc("/hub", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(hubHTML)
	})
	mux.HandleFunc("/console", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(hubHTML)
	})

	// Enterprise Demo Booking Portal
	mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if data, err := os.ReadFile("demo.html"); err == nil {
			w.Write(data)
			return
		}
		w.Write(demoHTML)
	})

	// Closed-Loop Supply Chain & Surat Jalan Simulator
	mux.HandleFunc("/simulator", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if data, err := os.ReadFile("simulator.html"); err == nil {
			w.Write(data)
			return
		}
		w.Write(simulatorHTML)
	})

	// Static Assets with 1-Year Immutable Caching & Native Range Request Support
	var assetHandler http.Handler
	if _, err := os.Stat("assets"); err == nil {
		assetHandler = http.StripPrefix("/assets/", http.FileServer(http.Dir("assets")))
	} else {
		assetHandler = http.FileServer(http.FS(assetsFS))
	}

	mux.Handle("/assets/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		assetHandler.ServeHTTP(w, r)
	}))

	// Company Profile Flagship Landing Page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if data, err := os.ReadFile("index.html"); err == nil {
			w.Write(data)
			return
		}
		w.Write(indexHTML)
	})

	addr := ":" + port
	log.Printf("Fledger Gateway listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, gzipMiddleware(mux)); err != nil {
		log.Fatalf("gateway server error: %v", err)
	}
}
