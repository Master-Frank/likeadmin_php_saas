package httpserver

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSONBodyLimitIsSmallerThanUpload(t *testing.T) {
	oldJSON, oldUp := jsonBodyBytes, maxBodyBytes
	jsonBodyBytes = 8
	maxBodyBytes = 32
	t.Cleanup(func() {
		jsonBodyBytes, maxBodyBytes = oldJSON, oldUp
	})
	var jsonErr, uploadErr error
	h := withLimits(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "/upload/") {
			uploadErr = err
			return
		}
		jsonErr = err
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/platformapi/auth.admin/add", strings.NewReader(strings.Repeat("x", 16))))
	if jsonErr == nil {
		t.Fatal("JSON over 1MiB-equivalent must fail")
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/platformapi/upload/image", strings.NewReader(strings.Repeat("x", 16))))
	if uploadErr != nil {
		t.Fatalf("upload under 50MiB-equivalent must pass, got %v", uploadErr)
	}
}

func TestIsUploadRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/tenantapi/upload/file", nil)
	if !isUploadRequest(req) {
		t.Fatal("upload")
	}
	req = httptest.NewRequest(http.MethodPost, "/platformapi/auth.admin/add", nil)
	if isUploadRequest(req) {
		t.Fatal("json")
	}
}
