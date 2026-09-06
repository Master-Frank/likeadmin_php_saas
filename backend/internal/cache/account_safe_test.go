package cache

import "testing"

func TestUserLoginSafeLock(t *testing.T) {
	ip := "203.0.113.9"
	RelieveUserLoginFail(ip)
	if !UserLoginSafe(ip) {
		t.Fatal("fresh ip should be safe")
	}
	for i := 0; i < userLoginSafeCount; i++ {
		RecordUserLoginFail(ip)
	}
	if UserLoginSafe(ip) {
		t.Fatal("should lock after 15 failures")
	}
	RelieveUserLoginFail(ip)
	if !UserLoginSafe(ip) {
		t.Fatal("relieved ip should be safe")
	}
}
