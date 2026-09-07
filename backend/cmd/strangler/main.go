package main

import (
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxUpload = 50 << 20

// Strangler front door: API prefixes go to the Go backend; static files and
// SPA shells are served from public_dir so PHP is no longer on the page path.
// LIKEADMIN_PHP_FALLBACK defaults off. Set it to 1 only while a leftover
// PHP path still needs a temporary proxy.
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
	public := resolvePublicDir()
	phpFallback := phpFallbackEnabled()
	goProxy := newForwardProxy(goURL)
	phpProxy := newForwardProxy(phpURL)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxUpload)
		if goAPI(r.URL.Path) {
			goProxy.ServeHTTP(w, r)
			return
		}
		if servePublic(w, r, public) {
			return
		}
		if phpFallback {
			phpProxy.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
	log.Printf("strangler listening on %s (api->%s public=%s php_fallback=%v)", listen, goURL, public, phpFallback)
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

func resolvePublicDir() string {
	if v := getenv("LIKEADMIN_PUBLIC", ""); v != "" {
		return v
	}
	for _, cand := range []string{
		"/workspace/server/public",
		filepath.Join("..", "..", "server", "public"),
		filepath.Join("server", "public"),
	} {
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			abs, err := filepath.Abs(cand)
			if err == nil {
				return abs
			}
			return cand
		}
	}
	return ""
}

func servePublic(w http.ResponseWriter, r *http.Request, public string) bool {
	if public == "" || w == nil || r == nil {
		return false
	}
	reqPath := path.Clean("/" + r.URL.Path)
	if !strings.HasPrefix(reqPath, "/") {
		reqPath = "/" + reqPath
	}
	rel := strings.TrimPrefix(reqPath, "/")
	full := filepath.Join(public, filepath.FromSlash(rel))
	root, err := filepath.Abs(public)
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(full)
	if err != nil {
		return false
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return false
	}
	if st, err := os.Stat(abs); err == nil && !st.IsDir() {
		http.ServeFile(w, r, abs)
		return true
	}
	for _, prefix := range []string{"/platform", "/admin", "/mobile", "/pc"} {
		if reqPath == prefix || strings.HasPrefix(reqPath, prefix+"/") {
			index := filepath.Join(public, strings.TrimPrefix(prefix, "/"), "index.html")
			if _, err := os.Stat(index); err == nil {
				http.ServeFile(w, r, index)
				return true
			}
		}
	}
	if reqPath == "/" {
		index := filepath.Join(public, "index.html")
		if _, err := os.Stat(index); err == nil {
			http.ServeFile(w, r, index)
			return true
		}
	}
	return false
}

func phpFallbackEnabled() bool {
	return getenv("LIKEADMIN_PHP_FALLBACK", "0") != "0"
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
