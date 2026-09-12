package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPHPFallbackDefaultOff(t *testing.T) {
	t.Setenv("LIKEADMIN_PHP_FALLBACK", "")
	if phpFallbackEnabled() {
		t.Fatal("production default must not proxy leftover paths to PHP")
	}
	t.Setenv("LIKEADMIN_PHP_FALLBACK", "0")
	if phpFallbackEnabled() {
		t.Fatal("explicit 0")
	}
	t.Setenv("LIKEADMIN_PHP_FALLBACK", "1")
	if !phpFallbackEnabled() {
		t.Fatal("explicit 1 should enable PHP fallback")
	}
}

func TestGoAPI(t *testing.T) {
	cases := map[string]bool{
		"/platformapi/login/account":            true,
		"/tenantapi/config/getConfig":           true,
		"/api/index/config":                     true,
		"/crontab":                              true,
		"/install":                              true,
		"/":                                     true,
		"/install/install.php":                  true,
		"/index.php/platformapi/login/account":  true,
		"/index.php/tenantapi/config/getConfig": true,
		"/index.php/api/index/config":           true,
		"/index.php/crontab":                    true,
		"/index.php":                            false,
		"/admin":                                false,
		"/mobile":                               false,
		"/resource/x.png":                       false,
	}
	for path, want := range cases {
		if got := goAPI(path); got != want {
			t.Fatalf("%s: got %v want %v", path, got, want)
		}
	}
}

func TestSetForwarded(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://pair1.likeadmin.test/api/index/config", nil)
	req.RemoteAddr = "10.1.2.3:54321"
	req.Host = "pair1.likeadmin.test"
	setForwarded(req, req.Host)
	if req.Header.Get("X-Forwarded-Host") != "pair1.likeadmin.test" {
		t.Fatalf("host=%s", req.Header.Get("X-Forwarded-Host"))
	}
	if req.Header.Get("X-Forwarded-Proto") != "http" {
		t.Fatalf("proto=%s", req.Header.Get("X-Forwarded-Proto"))
	}
	if req.Header.Get("X-Real-IP") != "10.1.2.3" {
		t.Fatalf("ip=%s", req.Header.Get("X-Real-IP"))
	}
	req.TLS = &tls.ConnectionState{}
	req.Header.Del("X-Forwarded-Proto")
	setForwarded(req, req.Host)
	if req.Header.Get("X-Forwarded-Proto") != "https" {
		t.Fatalf("tls proto=%s", req.Header.Get("X-Forwarded-Proto"))
	}
}

func TestNewForwardProxyKeepsHost(t *testing.T) {
	target, _ := url.Parse("http://127.0.0.1:8080")
	p := newForwardProxy(target)
	req := httptest.NewRequest(http.MethodGet, "http://pair1.likeadmin.test/api/index/config", nil)
	req.Host = "pair1.likeadmin.test"
	req.RemoteAddr = "127.0.0.1:9"
	p.Director(req)
	if req.Host != "pair1.likeadmin.test" {
		t.Fatalf("host rewritten to %s", req.Host)
	}
	if req.Header.Get("X-Forwarded-Host") != "pair1.likeadmin.test" {
		t.Fatalf("x-forwarded-host=%s", req.Header.Get("X-Forwarded-Host"))
	}
}

func TestServePublicSPA(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "admin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "admin", "index.html"), []byte("<html>admin-spa</html>"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "resource"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "resource", "x.txt"), []byte("static"), 0644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/login", nil)
	if !servePublic(rec, req, dir) || !strings.Contains(rec.Body.String(), "admin-spa") {
		t.Fatalf("spa: %s %s", rec.Result().Status, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/resource/x.txt", nil)
	if !servePublic(rec, req, dir) || rec.Body.String() != "static" {
		t.Fatalf("static: %s", rec.Body.String())
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/missing", nil)
	if servePublic(rec, req, dir) {
		t.Fatal("missing should fall through")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/../etc/passwd", nil)
	if servePublic(rec, req, dir) {
		t.Fatal("path escape")
	}
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/pages/news/news?id=1", nil)
	if !servePublic(rec, req, dir) {
		t.Fatal("pages should redirect to PC")
	}
	if rec.Code != http.StatusFound {
		t.Fatalf("pages status %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/pc/information" {
		t.Fatalf("pages location %q", loc)
	}
}
