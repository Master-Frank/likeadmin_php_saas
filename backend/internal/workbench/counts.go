package workbench

import (
	"strconv"
	"time"

	"likeadmin/backend/internal/cache"
)

const countTTL = 45 * time.Second

func CachedCount(key string, load func() int64) int64 {
	if raw, ok := cache.Get(key); ok {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return n
		}
	}
	n := load()
	cache.Set(key, strconv.FormatInt(n, 10), countTTL)
	return n
}

func IncrIfPresent(key string) {
	if _, ok := cache.Get(key); ok {
		cache.Incr(key)
	}
}

func TenantTotalKey() string { return "wb:tenants:total" }

func TenantTodayKey(now time.Time) string {
	return "wb:tenants:today:" + now.Format("20060102")
}

func UserTotalKey(tid uint) string {
	return "wb:users:" + strconv.FormatUint(uint64(tid), 10) + ":total"
}

func UserTodayKey(tid uint, now time.Time) string {
	return "wb:users:" + strconv.FormatUint(uint64(tid), 10) + ":today:" + now.Format("20060102")
}

func OnTenantCreated() {
	IncrIfPresent(TenantTotalKey())
	IncrIfPresent(TenantTodayKey(time.Now()))
}

func OnUserCreated(tid uint) {
	if tid == 0 {
		return
	}
	IncrIfPresent(UserTotalKey(tid))
	IncrIfPresent(UserTodayKey(tid, time.Now()))
}
