package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestCORSAllowMethods(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	CORS()(c)
	got := w.Header().Get("Access-Control-Allow-Methods")
	if got != "GET, POST, PATCH, PUT, DELETE, post, OPTIONS" {
		t.Fatalf("Allow-Methods %q", got)
	}
}

func TestPlatformRequestTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	q := func(raw string, body string) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/x"+raw, bytes.NewBufferString(body))
		if body != "" {
			c.Request.Header.Set("Content-Type", "application/json")
		}
		return platformRequestTenantID(c)
	}
	if q("?tenant_id=2", `{"tenant_id":9}`) != "2" {
		t.Fatal("query wins")
	}
	if q("", `{"tenant_id":9}`) != "9" {
		t.Fatal("body tenant_id")
	}
	if q("", `{"tenantId":7}`) != "7" {
		t.Fatal("body tenantId")
	}
	if q("", `{}`) != "" {
		t.Fatal("empty")
	}
}

func TestServeTenantErrorPage(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "error", "tenant"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "error", "tenant", "404.html"), []byte("<html>tenant-404</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	config.C.App.PublicDir = dir
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/admin", nil)
	if !serveTenantError(c, true, "404.html") {
		t.Fatal("page request should serve HTML")
	}
	if w.Body.String() != "<html>tenant-404</html>" {
		t.Fatalf("body %q", w.Body.String())
	}
	if serveTenantError(c, false, "404.html") {
		t.Fatal("API request should not serve HTML")
	}
}

func TestIsStaticPath(t *testing.T) {
	if !isStaticPath("/resource/image/a.png") || !isStaticPath("/uploads/x") || !isStaticPath("/static/a.js") {
		t.Fatal("static prefixes")
	}
	if !isStaticPath("/admin/assets/index-abc.js") || !isStaticPath("/platform/assets/app.css") {
		t.Fatal("hashed SPA assets")
	}
	if isStaticPath("/platform/") || isStaticPath("/admin") || isStaticPath("/platformapi/x") {
		t.Fatal("non-static")
	}
}

func TestStaticPathSkipsTenantLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	lock := filepath.Join(dir, "install.lock")
	if err := os.WriteFile(lock, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	oldLock, oldPub := config.C.App.InstallLock, config.C.App.PublicDir
	t.Cleanup(func() {
		config.C.App.InstallLock = oldLock
		config.C.App.PublicDir = oldPub
	})
	config.C.App.InstallLock = lock
	config.C.App.PublicDir = dir

	r := gin.New()
	r.Use(InstallAndTenant())
	r.GET("/resource/*any", func(c *gin.Context) { c.String(http.StatusOK, "img") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/resource/image/platformapi/default/login_image.png", nil))
	if w.Code != http.StatusOK || w.Body.String() != "img" {
		t.Fatalf("static must skip tenant resolve: %d %s", w.Code, w.Body.String())
	}
}
