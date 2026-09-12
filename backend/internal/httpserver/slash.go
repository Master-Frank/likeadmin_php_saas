package httpserver

import (
	"net/http"
	"strings"
)

// MergeSlashes collapses adjacent slashes in the request path before routing.
// nginx merge_slashes on does this by default; Gin Static("/resource") does not
// match "//resource/...", which is what PC getImageUrl builds when domain ends
// with "/" and the decorate uri starts with "/".
func MergeSlashes(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(http.NotFound)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.URL == nil {
			next.ServeHTTP(w, r)
			return
		}
		path := mergeSlashes(r.URL.Path)
		raw := r.URL.RawPath
		if raw != "" {
			raw = mergeSlashes(raw)
		}
		if path == r.URL.Path && raw == r.URL.RawPath {
			next.ServeHTTP(w, r)
			return
		}
		cp := r.Clone(r.Context())
		cp.URL.Path = path
		cp.URL.RawPath = raw
		if q := r.URL.RawQuery; q != "" {
			cp.RequestURI = path + "?" + q
		} else {
			cp.RequestURI = path
		}
		next.ServeHTTP(w, cp)
	})
}

func mergeSlashes(p string) string {
	if len(p) < 2 || !strings.Contains(p, "//") {
		return p
	}
	var b strings.Builder
	b.Grow(len(p))
	prevSlash := false
	for i := 0; i < len(p); i++ {
		c := p[i]
		if c == '/' {
			if prevSlash {
				continue
			}
			prevSlash = true
		} else {
			prevSlash = false
		}
		b.WriteByte(c)
	}
	return b.String()
}
