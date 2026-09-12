package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestAllowSkipsInDebug(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.App.Debug
	config.C.App.Debug = true
	t.Cleanup(func() { config.C.App.Debug = old })
	t.Setenv("LIKEADMIN_RATE_LIMIT", "")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
	if !Allow(c, KindLogin) {
		t.Fatal("debug should skip")
	}
}

func TestAllowEnforcedWhenFlagged(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.App.Debug
	config.C.App.Debug = true
	t.Cleanup(func() { config.C.App.Debug = old })
	t.Setenv("LIKEADMIN_RATE_LIMIT", "1")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
	c.Request.RemoteAddr = "10.0.0.1:9"
	for i := 0; i < 60; i++ {
		if !Allow(c, KindLogin) {
			t.Fatalf("hit %d should pass", i+1)
		}
	}
	if Allow(c, KindLogin) {
		t.Fatal("61st login should be limited")
	}
}

func TestAllowFailsClosedWhenRedisRequired(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.App.Debug
	config.C.App.Debug = true
	t.Cleanup(func() { config.C.App.Debug = old })
	t.Setenv("LIKEADMIN_RATE_LIMIT", "1")
	t.Setenv("LIKEADMIN_REQUIRE_REDIS", "1")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/login", nil)
	c.Request.RemoteAddr = "10.0.0.1:9"
	if Allow(c, KindLogin) {
		t.Fatal("redis-required rate limit must fail closed without Redis")
	}
}
