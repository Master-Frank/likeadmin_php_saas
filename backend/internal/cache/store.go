package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/metrics"

	"github.com/redis/go-redis/v9"
)

func ctx() context.Context {
	return context.Background()
}

type memItem struct {
	val string
	exp time.Time
}

var mem sync.Map

func memGet(key string) (string, bool) {
	v, ok := mem.Load(key)
	if !ok {
		return "", false
	}
	item := v.(memItem)
	if !item.exp.IsZero() && time.Now().After(item.exp) {
		mem.Delete(key)
		return "", false
	}
	return item.val, true
}

func memSet(key, val string, ttl time.Duration) {
	exp := time.Time{}
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	mem.Store(key, memItem{val: val, exp: exp})
}

func payload(val any) string {
	switch t := val.(type) {
	case string:
		return t
	case map[string]any, []any, []map[string]any, []string, []int:
		if s, err := phpSerialize(t); err == nil && s != "" {
			return s
		}
	}
	if s, err := phpSerialize(val); err == nil && strings.HasPrefix(s, "a:") {
		return s
	}
	b, err := json.Marshal(val)
	if err != nil {
		return ""
	}
	return string(b)
}

func isSecurityKey(key string) bool {
	switch {
	case strings.HasPrefix(key, "token_"),
		strings.HasPrefix(key, "admin_auth_"),
		strings.HasPrefix(key, "tenant_auth_"),
		key == "auth_cache_ver",
		strings.HasPrefix(key, "rl:"),
		strings.HasPrefix(key, "export_task_"),
		strings.HasPrefix(key, "export_file_"),
		strings.HasPrefix(key, "export_job_"),
		strings.HasPrefix(key, "export_lease_"),
		key == "export_jobs":
		return true
	default:
		return false
	}
}

func useMemFallback(key string) bool {
	if !isSecurityKey(key) {
		return true
	}
	if bootstrap.RDB != nil || config.RequireRedisConfigured() {
		return false
	}
	return true
}

func noteRedisErr(err error) {
	if err != nil && err != redis.Nil {
		metrics.AddRedisError()
	}
}

func Get(key string) (string, bool) {
	if bootstrap.RDB != nil {
		v, err := bootstrap.RDB.Get(ctx(), bootstrap.RedisKey(key)).Result()
		if err == nil {
			return v, true
		}
		noteRedisErr(err)
		if !useMemFallback(key) {
			return "", false
		}
	} else if !useMemFallback(key) {
		return "", false
	}
	return memGet(key)
}

func GetJSON(key string, dest any) bool {
	raw, ok := Get(key)
	if !ok || raw == "" {
		return false
	}
	if json.Unmarshal([]byte(raw), dest) == nil {
		return true
	}
	v, ok := phpUnserialize(raw)
	if !ok || v == nil {
		return false
	}
	b, err := json.Marshal(v)
	if err != nil {
		return false
	}
	return json.Unmarshal(b, dest) == nil
}

func Set(key string, val any, ttl time.Duration) {
	p := payload(val)
	if bootstrap.RDB != nil {
		if err := bootstrap.RDB.Set(ctx(), bootstrap.RedisKey(key), p, ttl).Err(); err == nil {
			return
		} else {
			noteRedisErr(err)
		}
		if !useMemFallback(key) {
			return
		}
	} else if !useMemFallback(key) {
		return
	}
	memSet(key, p, ttl)
}

func SetNX(key, val string, ttl time.Duration) bool {
	if bootstrap.RDB != nil {
		ok, err := bootstrap.RDB.SetNX(ctx(), bootstrap.RedisKey(key), val, ttl).Result()
		if err == nil {
			return ok
		}
		noteRedisErr(err)
		if !useMemFallback(key) {
			return false
		}
	} else if !useMemFallback(key) {
		return false
	}
	if _, ok := memGet(key); ok {
		return false
	}
	memSet(key, val, ttl)
	return true
}

func ListPush(key, val string) bool {
	if bootstrap.RDB != nil {
		if err := bootstrap.RDB.LPush(ctx(), bootstrap.RedisKey(key), val).Err(); err == nil {
			return true
		} else {
			noteRedisErr(err)
		}
		return false
	}
	return false
}

func ListPop(key string, wait time.Duration) (string, bool) {
	if bootstrap.RDB == nil {
		return "", false
	}
	res, err := bootstrap.RDB.BRPop(ctx(), wait, bootstrap.RedisKey(key)).Result()
	if err != nil || len(res) < 2 {
		noteRedisErr(err)
		return "", false
	}
	return res[1], true
}

// KeysPrefix returns logical (unprefixed) keys that start with prefix.
func KeysPrefix(prefix string) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(k string) {
		if k == "" {
			return
		}
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	if bootstrap.RDB != nil {
		match := bootstrap.RedisKey(prefix) + "*"
		rp := bootstrap.RedisKey("")
		var cursor uint64
		for {
			keys, next, err := bootstrap.RDB.Scan(ctx(), cursor, match, 100).Result()
			if err != nil {
				noteRedisErr(err)
				break
			}
			for _, k := range keys {
				add(strings.TrimPrefix(k, rp))
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	mem.Range(func(k, _ any) bool {
		s, ok := k.(string)
		if ok && strings.HasPrefix(s, prefix) {
			add(s)
		}
		return true
	})
	return out
}

func Del(key string) {
	if bootstrap.RDB != nil {
		_ = bootstrap.RDB.Del(ctx(), bootstrap.RedisKey(key)).Err()
	}
	mem.Delete(key)
}

// Flush clears this app's Redis keys (prefix + *) and the in-memory fallback.
// ThinkPHP Cache::clear() is process-wide; we keep the same logical wipe
// without FlushDB so a shared Redis is not emptied for other prefixes.
func Flush() {
	flushPrefixedRedis()
	mem.Range(func(k, _ any) bool {
		mem.Delete(k)
		return true
	})
}

func flushPrefixedRedis() {
	if bootstrap.RDB == nil {
		return
	}
	match := bootstrap.RedisKey("") + "*"
	var cursor uint64
	for {
		keys, next, err := bootstrap.RDB.Scan(ctx(), cursor, match, 200).Result()
		if err != nil {
			break
		}
		if len(keys) > 0 {
			_ = bootstrap.RDB.Del(ctx(), keys...).Err()
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
}

func DelPrefix(prefix string) {
	if prefix == "" {
		return
	}
	if bootstrap.RDB != nil {
		match := bootstrap.RedisKey(prefix) + "*"
		var cursor uint64
		for {
			keys, next, err := bootstrap.RDB.Scan(ctx(), cursor, match, 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = bootstrap.RDB.Del(ctx(), keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
	}
	mem.Range(func(k, _ any) bool {
		if s, ok := k.(string); ok && strings.HasPrefix(s, prefix) {
			mem.Delete(k)
		}
		return true
	})
}

func AuthCacheVer() string {
	raw, ok := Get("auth_cache_ver")
	if !ok || raw == "" {
		return "0"
	}
	return strings.TrimSpace(raw)
}

func BumpAuthCache() {
	n := Incr("auth_cache_ver")
	if n <= 0 {
		Set("auth_cache_ver", "1", 0)
	}
}

func ClearAdminAuthCache(adminID uint) {
	if adminID > 0 {
		id := strconv.FormatUint(uint64(adminID), 10)
		ver := AuthCacheVer()
		Del("admin_auth_url_" + id)
		Del("admin_auth_url_" + id + ":" + ver)
		Del("tenant_auth_url_" + id)
	}
	BumpAuthCache()
}

func Incr(key string) int64 {
	if bootstrap.RDB != nil {
		n, err := bootstrap.RDB.Incr(ctx(), bootstrap.RedisKey(key)).Result()
		if err == nil {
			return n
		}
		noteRedisErr(err)
		if !useMemFallback(key) {
			return -1
		}
	} else if !useMemFallback(key) {
		return -1
	}
	raw, _ := memGet(key)
	n := utilParseInt64(raw) + 1
	memSet(key, jsonNumber(n), 0)
	return n
}

const incrExpireLua = `
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return n
`

func IncrExpire(key string, ttl time.Duration) int64 {
	sec := int64(ttl / time.Second)
	if sec <= 0 {
		sec = 60
	}
	if bootstrap.RDB != nil {
		n, err := bootstrap.RDB.Eval(ctx(), incrExpireLua, []string{bootstrap.RedisKey(key)}, sec).Int64()
		if err == nil {
			return n
		}
		noteRedisErr(err)
		if !useMemFallback(key) {
			return -1
		}
	} else if !useMemFallback(key) {
		return -1
	}
	n := Incr(key)
	if n == 1 {
		Expire(key, ttl)
	}
	return n
}

func Expire(key string, ttl time.Duration) {
	if bootstrap.RDB != nil {
		_ = bootstrap.RDB.Expire(ctx(), bootstrap.RedisKey(key), ttl).Err()
	}
	if raw, ok := memGet(key); ok {
		memSet(key, raw, ttl)
	}
}

func utilParseInt64(s string) int64 {
	var n int64
	_ = json.Unmarshal([]byte(s), &n)
	return n
}

func jsonNumber(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
