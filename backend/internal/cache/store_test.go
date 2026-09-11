package cache

import "testing"

func TestFlush(t *testing.T) {
	Set("keep_before_flush", "1", 0)
	Set("sms_code_101_1", "1234", 0)
	Flush()
	if _, ok := Get("keep_before_flush"); ok {
		t.Fatal("flush should drop mem keys")
	}
	if _, ok := Get("sms_code_101_1"); ok {
		t.Fatal("sms cache should drop")
	}
}

func TestDelPrefix(t *testing.T) {
	Set("admin_auth_url_1", "a", 0)
	Set("admin_auth_all", "b", 0)
	Set("other_key", "c", 0)
	DelPrefix("admin_auth_")
	if _, ok := Get("admin_auth_url_1"); ok {
		t.Fatal("prefix key remains")
	}
	if _, ok := Get("admin_auth_all"); ok {
		t.Fatal("prefix all remains")
	}
	if _, ok := Get("other_key"); !ok {
		t.Fatal("unrelated key deleted")
	}
	Del("other_key")
}

func TestSecurityKeysSkipMemWhenRedisRequired(t *testing.T) {
	t.Setenv("LIKEADMIN_REQUIRE_REDIS", "1")
	Set("rl:login:1.1.1.1", "9", 0)
	if _, ok := Get("rl:login:1.1.1.1"); ok {
		t.Fatal("rate-limit keys must not use process memory when Redis is required")
	}
	if n := Incr("rl:login:1.1.1.1"); n >= 0 {
		t.Fatalf("incr must fail closed, got %d", n)
	}
	Set("boot:public", "ok", 0)
	if _, ok := Get("boot:public"); !ok {
		t.Fatal("public cache may still use local memory")
	}
	Del("boot:public")
}
