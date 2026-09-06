package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

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
	goProxy := httputil.NewSingleHostReverseProxy(goURL)
	phpProxy := httputil.NewSingleHostReverseProxy(phpURL)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if goAPI(r.URL.Path) {
			goProxy.ServeHTTP(w, r)
			return
		}
		phpProxy.ServeHTTP(w, r)
	})
	log.Printf("strangler listening on %s (api->%s other->%s)", listen, goURL, phpURL)
	log.Fatal(http.ListenAndServe(listen, nil))
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
