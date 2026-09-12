package performance_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/metrics"

	"github.com/gin-gonic/gin"
)

func TestQueryStatsAttachToRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(metrics.Middleware())
	r.GET("/healthz", func(c *gin.Context) {
		st := metrics.FromGin(c)
		if st == nil {
			t.Fatal("stats missing")
		}
		c.String(http.StatusOK, "ok")
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.ServeHTTP(w, req)
	if w.Code != 200 || w.Body.String() != "ok" {
		t.Fatalf("healthz %d %s", w.Code, w.Body.String())
	}
}

func TestExportCapsAreConfigured(t *testing.T) {
	if config.C.Project.Lists.ExportRows() > 10000 && config.C.Project.Lists.ExportMaxRows == 0 {
		t.Fatal("zero config should still default")
	}
	if config.C.Project.Lists.ExportRows() <= 0 || config.C.Project.Lists.ExportPages() <= 0 {
		t.Fatal("export caps")
	}
}
