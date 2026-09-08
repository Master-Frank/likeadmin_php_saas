package middleware

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
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

func TestDemoGuardMatchesPHPAblePost(t *testing.T) {
	if demoGuardBlocks("POST", "platformapi", "login", "logout") {
		t.Fatal("platform logout must stay allowed")
	}
	if demoGuardBlocks("POST", "tenantapi", "Login", "Logout") {
		t.Fatal("tenant logout must stay allowed (case-insensitive URI)")
	}
	if demoGuardBlocks("POST", "platformapi", "login", "account") {
		t.Fatal("login/account must stay allowed")
	}
	if !demoGuardBlocks("POST", "platformapi", "auth.admin", "add") {
		t.Fatal("platform writes must be blocked")
	}
	if !demoGuardBlocks("POST", "tenantapi", "user.user", "adjustmoney") {
		t.Fatal("tenant writes must be blocked")
	}
	if demoGuardBlocks("POST", "api", "login", "register") {
		t.Fatal("user-app POSTs are not gated by PHP CheckDemoMiddleware")
	}
	if demoGuardBlocks("POST", "api", "login", "mnplogin") {
		t.Fatal("user-app wechat login must not be gated")
	}
	if demoGuardBlocks("PUT", "platformapi", "auth.admin", "edit") {
		t.Fatal("PHP only inspects POST")
	}
	if demoGuardBlocks("GET", "platformapi", "auth.admin", "lists") {
		t.Fatal("GET must pass")
	}
}

func TestDemoGuardHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.Project.DemoEnv
	config.C.Project.DemoEnv = true
	t.Cleanup(func() { config.C.Project.DemoEnv = old })

	hit := func(method, app, ctrl, action string) (aborted bool, body string) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/", nil)
		ctxutil.Set(c, &ctxutil.RequestMeta{App: app, Controller: ctrl, Action: action})
		DemoGuard()(c)
		return c.IsAborted(), w.Body.String()
	}

	if aborted, body := hit("POST", "platformapi", "login", "logout"); aborted || strings.Contains(body, "演示环境") {
		t.Fatalf("logout blocked: aborted=%v body=%s", aborted, body)
	}
	aborted, body := hit("POST", "platformapi", "auth.admin", "add")
	if !aborted || !strings.Contains(body, "演示环境不支持修改数据，请下载源码本地部署体验") {
		t.Fatalf("admin add should block: aborted=%v body=%s", aborted, body)
	}
	if aborted, _ := hit("POST", "api", "login", "register"); aborted {
		t.Fatal("api register should pass")
	}

	config.C.Project.DemoEnv = false
	if aborted, _ := hit("POST", "platformapi", "auth.admin", "add"); aborted {
		t.Fatal("demo off should pass writes")
	}
}

func TestLoginIPChanged(t *testing.T) {
	if !loginIPChanged("1.1.1.1", "2.2.2.2") {
		t.Fatal("different IP should force re-login")
	}
	if loginIPChanged("1.1.1.1", "1.1.1.1") {
		t.Fatal("same IP should pass")
	}
	if !loginIPChanged("", "127.0.0.1") {
		t.Fatal("empty login_ip should mismatch a real client IP, matching PHP")
	}
	if loginIPChanged("", "") {
		t.Fatal("both empty should pass")
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
