package ctxutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClientIPPrefersXRealIPIgnoresXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("LIKEADMIN_TRUSTED_PROXIES", "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "127.0.0.1:4321"
	c.Request.Header.Set("X-Forwarded-For", "1.2.3.4")
	c.Request.Header.Set("X-Real-IP", "10.0.0.9")
	if got := ClientIP(c); got != "10.0.0.9" {
		t.Fatalf("real-ip %q", got)
	}
	c.Request.Header.Del("X-Real-IP")
	if got := ClientIP(c); got != "127.0.0.1" {
		t.Fatalf("remote %q", got)
	}
}

func TestClientIPRejectsSpoofedRealIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("LIKEADMIN_TRUSTED_PROXIES", "127.0.0.0/8")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "8.8.8.8:4321"
	c.Request.Header.Set("X-Real-IP", "10.0.0.9")
	c.Request.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := ClientIP(c); got != "8.8.8.8" {
		t.Fatalf("direct access must ignore spoofed real-ip, got %q", got)
	}
}

func TestClientIPTrustsConfiguredProxyCIDR(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("LIKEADMIN_TRUSTED_PROXIES", "10.0.0.0/8")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "10.1.2.3:80"
	c.Request.Header.Set("X-Real-IP", "203.0.113.9")
	if got := ClientIP(c); got != "203.0.113.9" {
		t.Fatalf("trusted proxy %q", got)
	}
}
