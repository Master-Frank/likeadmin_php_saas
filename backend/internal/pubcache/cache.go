package pubcache

import (
	"strconv"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

const TTL = 30 * time.Second

func key(tid uint, kind, extra string) string {
	return "pub:" + strconv.FormatUint(uint64(tid), 10) + ":" + kind + ":" + extra
}

func GetJSON(tid uint, kind, extra string, dest any) bool {
	return cache.GetJSON(key(tid, kind, extra), dest)
}

func Set(tid uint, kind, extra string, val any, ttl time.Duration) {
	if ttl <= 0 {
		ttl = TTL
	}
	cache.Set(key(tid, kind, extra), val, ttl)
}

func Invalidate(tid uint, kinds ...string) {
	id := strconv.FormatUint(uint64(tid), 10)
	if len(kinds) == 0 {
		cache.DelPrefix("pub:" + id + ":")
		return
	}
	for _, kind := range kinds {
		cache.DelPrefix("pub:" + id + ":" + kind + ":")
	}
}

func TenantID(c *gin.Context) uint {
	if c == nil {
		return 0
	}
	return ctxutil.Get(c).TenantID
}
