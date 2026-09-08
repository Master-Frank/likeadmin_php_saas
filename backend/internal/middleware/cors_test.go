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
