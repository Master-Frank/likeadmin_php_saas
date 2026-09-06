package cache

import (
	"context"
	"encoding/json"
	"time"

	"likeadmin/backend/internal/bootstrap"

	"github.com/redis/go-redis/v9"
)

func ctx() context.Context { return context.Background() }

func Get(key string) (string, bool) {
	if bootstrap.RDB == nil {
		return "", false
	}
	v, err := bootstrap.RDB.Get(ctx(), bootstrap.RedisKey(key)).Result()
	if err == redis.Nil || err != nil {
		return "", false
	}
	return v, true
}

func GetJSON(key string, dest any) bool {
	raw, ok := Get(key)
	if !ok || raw == "" {
		return false
	}
	return json.Unmarshal([]byte(raw), dest) == nil
}

func Set(key string, val any, ttl time.Duration) {
	if bootstrap.RDB == nil {
		return
	}
	var payload string
	switch t := val.(type) {
	case string:
		payload = t
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return
		}
		payload = string(b)
	}
	_ = bootstrap.RDB.Set(ctx(), bootstrap.RedisKey(key), payload, ttl).Err()
}

func Del(key string) {
	if bootstrap.RDB == nil {
		return
	}
	_ = bootstrap.RDB.Del(ctx(), bootstrap.RedisKey(key)).Err()
}

func Incr(key string) int64 {
	if bootstrap.RDB == nil {
		return 0
	}
	n, _ := bootstrap.RDB.Incr(ctx(), bootstrap.RedisKey(key)).Result()
	return n
}

func Expire(key string, ttl time.Duration) {
	if bootstrap.RDB == nil {
		return
	}
	_ = bootstrap.RDB.Expire(ctx(), bootstrap.RedisKey(key), ttl).Err()
}
