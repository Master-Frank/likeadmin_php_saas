package tenantapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

func TestRequirePlatformTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourcePlatform})
	if requirePlatformTenant(c) {
		t.Fatal("platform without tenant_id should fail")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourcePlatform, TenantID: 1})
	if !requirePlatformTenant(c) {
		t.Fatal("platform with tenant_id should pass")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{})
	if !requirePlatformTenant(c) {
		t.Fatal("non-platform should pass")
	}
}

func TestGuardTenantWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourcePlatform})
	if guardTenantWrite(c) {
		t.Fatal("platform without tenant_id should fail")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourcePlatform, TenantID: 1})
	if !guardTenantWrite(c) {
		t.Fatal("platform with tenant_id should pass")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant})
	if guardTenantWrite(c) {
		t.Fatal("tenant without tenant_id should fail")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant, TenantID: 2})
	if !guardTenantWrite(c) {
		t.Fatal("tenant with tenant_id should pass")
	}
}
