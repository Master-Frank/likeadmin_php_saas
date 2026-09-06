package cache

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"likeadmin/backend/internal/bootstrap"
)

func ctx() context.Context { return context.Background() }

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

func Get(key string) (string, bool) {
	if bootstrap.RDB != nil {
		v, err := bootstrap.RDB.Get(ctx(), bootstrap.RedisKey(key)).Result()
		if err == nil {
			return v, true
		}
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
		}
	}
	memSet(key, p, ttl)
}

func Del(key string) {
	if bootstrap.RDB != nil {
		_ = bootstrap.RDB.Del(ctx(), bootstrap.RedisKey(key)).Err()
	}
	mem.Delete(key)
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

func ClearAdminAuthCache(adminID uint) {
	if adminID > 0 {
		id := strconv.FormatUint(uint64(adminID), 10)
		Del("admin_auth_url_" + id)
		Del("tenant_auth_url_" + id)
	}
	DelPrefix("admin_auth_")
	DelPrefix("tenant_auth_")
}

func Incr(key string) int64 {
	if bootstrap.RDB != nil {
		n, err := bootstrap.RDB.Incr(ctx(), bootstrap.RedisKey(key)).Result()
		if err == nil {
			return n
		}
	}
	raw, _ := memGet(key)
	n := utilParseInt64(raw) + 1
	memSet(key, jsonNumber(n), 0)
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
