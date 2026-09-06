package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

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
