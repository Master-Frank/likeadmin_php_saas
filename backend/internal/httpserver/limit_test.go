package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithLimitsRejectsOversizedBody(t *testing.T) {
	old := maxBodyBytes
	maxBodyBytes = 8
	t.Cleanup(func() { maxBodyBytes = old })
	h := withLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("expected MaxBytesReader error")
		}
	}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(strings.Repeat("x", 32)))
	h.ServeHTTP(httptest.NewRecorder(), req)
}

func TestIsExportRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/platformapi/download/export?file=a", nil)
	if !isExportRequest(req) {
		t.Fatal("download/export")
	}
	req = httptest.NewRequest(http.MethodGet, "/platformapi/auth.admin/lists?export=2", nil)
	if !isExportRequest(req) {
		t.Fatal("export=2")
	}
	req = httptest.NewRequest(http.MethodGet, "/platformapi/auth.admin/lists", nil)
	if isExportRequest(req) {
		t.Fatal("normal lists")
	}
}
