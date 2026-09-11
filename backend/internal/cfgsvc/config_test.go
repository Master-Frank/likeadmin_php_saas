package cfgsvc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

func TestGetStringEmptyDefUsesProjectFallback(t *testing.T) {
	config.C.Project.Platform = map[string]string{"name": "SaaS平台端"}
	got := GetString(nil, "platform", "name", "")
	if got != "SaaS平台端" {
		t.Fatalf("got %q", got)
	}
	got = GetString(nil, "platform", "name", "caller-default")
	if got != "caller-default" {
		t.Fatalf("explicit default should win, got %q", got)
	}
}

func TestGetManyUsesProjectFallback(t *testing.T) {
	config.C.Project.Website = map[string]string{"shop_name": "likeadmin"}
	got := GetMany(nil, "website", []string{"shop_name", "missing"})
	if got["shop_name"] != "likeadmin" {
		t.Fatalf("%v", got)
	}
}

func TestRequestLocalDedupes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{})
	localSet(c, "website", "shop_name", localEntry{val: "once"})
	if GetString(c, "website", "shop_name", "") != "once" {
		t.Fatal("local hit")
	}
}

func TestSetInvalidatesRedisAndBoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{TenantID: 3, Source: ctxutil.SourceTenant})
	cache.Set("cfg:tenant:3:website:shop_name", `"old"`, 0)
	cache.Set("boot:3:0", `{}`, 0)
	t.Cleanup(func() {
		cache.Del("cfg:tenant:3:website:shop_name")
		cache.DelPrefix("boot:3:")
		cache.Del("bootver:3")
	})
	invalidateCfg(c, false, 3, "website", "shop_name")
	if _, ok := cache.Get("cfg:tenant:3:website:shop_name"); ok {
		t.Fatal("cfg key should drop")
	}
	if _, ok := cache.Get("boot:3:0"); ok {
		t.Fatal("boot bundle should drop")
	}
	if BootVersion(3) == "0" {
		t.Fatal("boot version should bump")
	}
}

func TestGetDoesNotCacheUnavailableDB(t *testing.T) {
	if bootstrap.DB != nil {
		t.Skip("live DB would load real config rows")
	}
	key := redisKey(false, 0, "website", "shop_name")
	cache.Del(key)
	t.Cleanup(func() { cache.Del(key) })
	_ = Get(nil, "website", "shop_name", "fallback")
	if _, ok := cache.Get(key); ok {
		t.Fatal("sql/db errors must not be cached as misses")
	}
}

func TestSensitiveCfgStaysLocal(t *testing.T) {
	if !sensitiveCfg("storage", "qiniu") || sensitiveCfg("storage", "default") {
		t.Fatal("storage secrets")
	}
	if !sensitiveCfg("sms", "aliyun") || sensitiveCfg("website", "shop_name") {
		t.Fatal("sms/website")
	}
	if !sensitiveCfg("oa_setting", "token") || !sensitiveCfg("oa_setting", "encoding_aes_key") {
		t.Fatal("oa token/aes")
	}
	if !sensitiveCfg("mnp_setting", "token") || sensitiveCfg("oa_setting", "app_id") {
		t.Fatal("mnp token vs app_id")
	}
}
