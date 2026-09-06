package storage

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestFetchLocal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("jpeg-bytes"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	config.C.App.PublicDir = dir
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	got, err := Fetch(c, srv.URL+"/a.jpg", "uploads/user/avatar/t.jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if got.URI != "uploads/user/avatar/t.jpeg" || got.Engine != "local" {
		t.Fatalf("%+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "uploads/user/avatar/t.jpeg"))
	if err != nil || string(raw) != "jpeg-bytes" {
		t.Fatalf("saved %q %v", raw, err)
	}
}
