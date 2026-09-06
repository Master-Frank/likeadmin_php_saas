package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

const maxUpload = 50 << 20

// Strangler front door: API prefixes go to the Go backend, everything else to PHP.
func main() {
	listen := getenv("LIKEADMIN_STRANGLER", "127.0.0.1:8090")
	goURL, err := url.Parse(getenv("LIKEADMIN_GO", "http://127.0.0.1:8080"))
	if err != nil {
		log.Fatal(err)
	}
	phpURL, err := url.Parse(getenv("LIKEADMIN_PHP", "http://127.0.0.1:8000"))
	if err != nil {
		log.Fatal(err)
	}
	goProxy := newForwardProxy(goURL)
	phpProxy := newForwardProxy(phpURL)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if goAPI(r.URL.Path) {
			goProxy.ServeHTTP(w, r)
			return
		}
		phpProxy.ServeHTTP(w, r)
	})
	log.Printf("strangler listening on %s (api->%s other->%s)", listen, goURL, phpURL)
	log.Fatal(http.ListenAndServe(listen, nil))
}

func newForwardProxy(target *url.URL) *httputil.ReverseProxy {
	p := httputil.NewSingleHostReverseProxy(target)
	orig := p.Director
	p.Director = func(req *http.Request) {
		host := req.Host
		orig(req)
		req.Host = host
		setForwarded(req, host)
	}
	return p
}

func setForwarded(req *http.Request, host string) {
	if req.Header.Get("X-Forwarded-Host") == "" && host != "" {
		req.Header.Set("X-Forwarded-Host", host)
	}
	if req.Header.Get("X-Forwarded-Proto") == "" {
		proto := "http"
		if req.TLS != nil {
			proto = "https"
		}
		req.Header.Set("X-Forwarded-Proto", proto)
	}
	if req.Header.Get("X-Real-IP") == "" {
		if ip, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			req.Header.Set("X-Real-IP", ip)
		} else if req.RemoteAddr != "" {
			req.Header.Set("X-Real-IP", req.RemoteAddr)
		}
	}
}

func goAPI(path string) bool {
	for _, p := range []string{"/platformapi/", "/tenantapi/", "/api/", "/crontab", "/install"} {
		if strings.HasPrefix(path, p) || path == strings.TrimSuffix(p, "/") {
			return true
		}
	}
	return false
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
