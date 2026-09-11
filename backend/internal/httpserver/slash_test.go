package httpserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMergeSlashes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"/", "/"},
		{"/resource/a.png", "/resource/a.png"},
		{"//resource/image/tenantapi/default/banner003.png", "/resource/image/tenantapi/default/banner003.png"},
		{"/foo//bar/", "/foo/bar/"},
		{"///resource//a.png", "/resource/a.png"},
		{"/pc/", "/pc/"},
	}
	for _, tc := range cases {
		if got := mergeSlashes(tc.in); got != tc.want {
			t.Fatalf("mergeSlashes(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestMergeSlashesGinStatic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "banner.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Static("/resource", dir)
	h := MergeSlashes(r)

	hit := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1"+path, nil)
		req.URL.Path = path
		req.RequestURI = path
		h.ServeHTTP(w, req)
		return w
	}

	ok := hit("/resource/banner.png")
	if ok.Code != http.StatusOK || ok.Body.String() != "PNGDATA" {
		t.Fatalf("single slash: %d %q", ok.Code, ok.Body.String())
	}

	dbl := hit("//resource/banner.png")
	if dbl.Code != http.StatusOK || dbl.Body.String() != "PNGDATA" {
		t.Fatalf("double slash: %d %q", dbl.Code, dbl.Body.String())
	}

	miss := hit("//resource/missing.png")
	if miss.Code != http.StatusNotFound {
		t.Fatalf("missing: %d", miss.Code)
	}
}
