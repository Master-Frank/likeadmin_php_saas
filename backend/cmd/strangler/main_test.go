package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestGoAPI(t *testing.T) {
	cases := map[string]bool{
		"/platformapi/login/account":  true,
		"/tenantapi/config/getConfig": true,
		"/api/index/config":           true,
		"/crontab":                    true,
		"/install":                    true,
		"/admin":                      false,
		"/mobile":                     false,
		"/resource/x.png":             false,
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
