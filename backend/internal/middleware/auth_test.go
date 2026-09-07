package middleware

import (
	"reflect"
	"testing"

	"likeadmin/backend/internal/cache"
)

func TestFormatURIPerms(t *testing.T) {
	if got := formatURI("auth.admin/lists"); got != "auth.admin/lists" {
		t.Fatalf("%q", got)
	}
	if got := formatURI("user.user/adjustMoney"); got != "user.user/adjustmoney" {
		t.Fatalf("%q", got)
	}
	if got := formatURI("user.user/adjust_money"); got != "user.user/adjustmoney" {
		t.Fatalf("%q", got)
	}
	if !containsURI([]string{"user.user/adjustmoney"}, "user.user/adjustMoney") {
		t.Fatal("camel action should match")
	}
}

func TestRejectWrongTenant(t *testing.T) {
	if rejectWrongTenant(false, 1, 2) {
		t.Fatal("optional login should allow a stale cross-tenant token")
	}
	if !rejectWrongTenant(true, 1, 2) {
		t.Fatal("required login should reject a cross-tenant token")
	}
	if rejectWrongTenant(true, 1, 1) {
		t.Fatal("same tenant should pass")
	}
	if rejectWrongTenant(true, 1, 0) {
		t.Fatal("unset host tenant should pass")
	}
}

func TestPermsFingerprint(t *testing.T) {
	a := permsFingerprint([]string{"b", "a"})
	b := permsFingerprint([]string{"a", "b"})
	if a != b || a == "" {
		t.Fatalf("fingerprint should be order-stable: %s %s", a, b)
	}
	if permsFingerprint([]string{"a"}) == permsFingerprint([]string{"a", "b"}) {
		t.Fatal("different perms should change fingerprint")
	}
}

func TestCachedURIListInvalidates(t *testing.T) {
	cache.DelPrefix("admin_auth_")
	t.Cleanup(func() { cache.DelPrefix("admin_auth_") })
	storeURIList("admin_auth_all", []string{"old/path"})
	cache.Set("admin_auth_all_md5", "stale", 0)
	storeURIList("admin_auth_url_9", []string{"old/url"})
	got := cachedURIList("admin_auth_all", func() []string { return []string{"auth.admin/lists"} })
	if len(got) != 1 || got[0] != "auth.admin/lists" {
		t.Fatalf("live perms=%v", got)
	}
	if loadURIList("admin_auth_url_9") != nil {
		t.Fatal("stale admin url cache should drop when perms fingerprint changes")
	}
}

func TestAuthURIListCache(t *testing.T) {
	key := "admin_auth_url_test"
	cache.Del(key)
	t.Cleanup(func() { cache.Del(key) })
	if loadURIList(key) != nil {
		t.Fatal("empty cache should miss")
	}
	storeURIList(key, nil)
	if loadURIList(key) != nil {
		t.Fatal("empty list should not be cached")
	}
	want := []string{"auth.admin/lists", "user.user/adjustmoney"}
	storeURIList(key, want)
	got := loadURIList(key)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
