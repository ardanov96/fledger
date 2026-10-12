package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

//go:embed index.html
var indexHTML []byte

//go:embed hub.html
var hubHTML []byte

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

	// Company Profile Flagship Landing Page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexHTML)
	})

	addr := ":" + port
	log.Printf("Fledger Gateway listening on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("gateway server error: %v", err)
	}
}
