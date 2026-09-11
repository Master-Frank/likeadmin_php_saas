package ratelimit

import (
	"os"
	"strconv"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

const (
	KindLogin    = "login"
	KindSMS      = "sms"
	KindUpload   = "upload"
	KindPay      = "pay"
	KindInstall  = "install"
	KindGenerate = "generate"
)

func limitFor(kind string) int {
	switch kind {
	case KindLogin:
		return 60
	case KindSMS:
		return 20
	case KindUpload:
		return 60
	case KindPay:
		return 30
	case KindInstall:
		return 10
	case KindGenerate:
		return 10
	default:
		return 120
	}
}

// Allow increments a per-IP Redis counter. Debug builds skip unless
// LIKEADMIN_RATE_LIMIT=1 so pair/tests stay deterministic.
func Allow(c *gin.Context, kind string) bool {
	if c == nil {
		return true
	}
	if config.C.App.Debug && os.Getenv("LIKEADMIN_RATE_LIMIT") != "1" {
		return true
	}
	limit := limitFor(kind)
	ip := ctxutil.ClientIP(c)
	if ip == "" {
		ip = "unknown"
	}
	key := "rl:" + kind + ":" + ip
	n := cache.Incr(key)
	if n == 1 {
		cache.Expire(key, time.Minute)
	}
	if n > int64(limit) {
		response.Fail(c, "请求过于频繁，请稍后再试")
		return false
	}
	return true
}

func Remaining(kind, ip string) int64 {
	raw, ok := cache.Get("rl:" + kind + ":" + ip)
	if !ok || raw == "" {
		return int64(limitFor(kind))
	}
	n, _ := strconv.ParseInt(raw, 10, 64)
	left := int64(limitFor(kind)) - n
	if left < 0 {
		return 0
	}
	return left
}
