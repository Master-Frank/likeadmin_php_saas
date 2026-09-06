package cache

import (
	"time"

	"likeadmin/backend/internal/util"
)

const (
	userLoginSafeCount  = 15
	userLoginSafeMinute = 15
)

func userLoginSafeKey(ip string) string {
	return "UserAccountSafeCache" + ip
}

func UserLoginSafe(ip string) bool {
	raw, ok := Get(userLoginSafeKey(ip))
	if !ok || raw == "" {
		return true
	}
	n := utilParseInt64(raw)
	if n == 0 {
		n = int64(util.ToInt(raw))
	}
	return n < userLoginSafeCount
}

func RecordUserLoginFail(ip string) {
	key := userLoginSafeKey(ip)
	if _, ok := Get(key); ok {
		Incr(key)
		return
	}
	Set(key, 1, time.Duration(userLoginSafeMinute)*time.Minute)
}

func RelieveUserLoginFail(ip string) {
	Del(userLoginSafeKey(ip))
}

func UserLoginSafeHint() string {
	return "密码连续" + util.ToString(userLoginSafeCount) + "次输入错误，请" + util.ToString(userLoginSafeMinute) + "分钟后重试"
}
