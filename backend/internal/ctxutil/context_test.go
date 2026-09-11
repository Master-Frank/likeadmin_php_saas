package ctxutil

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestClientIPPrefersXRealIPIgnoresXFF(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.RemoteAddr = "192.168.1.8:4321"
	c.Request.Header.Set("X-Forwarded-For", "1.2.3.4")
	c.Request.Header.Set("X-Real-IP", "10.0.0.9")
	if got := ClientIP(c); got != "10.0.0.9" {
		t.Fatalf("real-ip %q", got)
	}
	c.Request.Header.Del("X-Real-IP")
	if got := ClientIP(c); got != "192.168.1.8" {
		t.Fatalf("remote %q", got)
	}
}
