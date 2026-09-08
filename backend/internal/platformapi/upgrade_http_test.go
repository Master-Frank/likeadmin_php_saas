package platformapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/upgrade"

	"github.com/gin-gonic/gin"
)

func writeUpgradeZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "package.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, body); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return path
}

func TestUpgradeDoHTTPFixture(t *testing.T) {
	zipPath := writeUpgradeZip(t, map[string]string{
		"project/server/public/upgrade-http.txt": "http-ok",
	})
	fixture := t.TempDir()
	lists := `{"code":1,"data":{"count":2,"lists":[{"id":99,"version_no":"9.9.9"},{"id":1,"version_no":"1.0.5"}]}}`
	verify := `{"code":1,"data":{"has_permission":true,"link":"file://` + zipPath + `","msg":""}}`
	if err := os.WriteFile(filepath.Join(fixture, "lists.json"), []byte(lists), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "verify.json"), []byte(verify), 0644); err != nil {
		t.Fatal(err)
	}

	tree := t.TempDir()
	public := filepath.Join(tree, "proj", "server", "public")
	if err := os.MkdirAll(public, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "proj", "server", "upgrade"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "proj", "server", "upgrade", "version.json"), []byte(`{"version":"1.0.5"}`), 0644); err != nil {
		t.Fatal(err)
	}

	oldPub := config.C.App.PublicDir
	oldDB := bootstrap.DB
	t.Cleanup(func() {
		config.C.App.PublicDir = oldPub
		bootstrap.DB = oldDB
	})
	config.C.App.PublicDir = public
	bootstrap.DB = nil

	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", fixture)
	t.Setenv("LIKEADMIN_OPEN_BASEDIR", "")
	t.Setenv("PHP_OPEN_BASEDIR", "")
	cache.Del("version_lists")
	cache.Del("version_lists1")

	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]any{"id": 99, "update_type": 1})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/upgrade.upgrade/upgrade", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Host = "pair1.likeadmin.test"
	UpgradeDo(c)

	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code != 1 || wrap.Msg != "更新成功" {
		t.Fatalf("upgrade http: %+v body=%s", wrap, w.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(tree, "proj", "public", "upgrade-http.txt"))
	if err != nil || string(got) != "http-ok" {
		t.Fatalf("probe %q err=%v", got, err)
	}
	if upgrade.LocalVersion() != "9.9.9" {
		t.Fatalf("version %s", upgrade.LocalVersion())
	}
}

func TestUpgradeDoHTTPDenied(t *testing.T) {
	fixture := t.TempDir()
	lists := `{"code":1,"data":{"count":2,"lists":[{"id":99,"version_no":"9.9.9"},{"id":1,"version_no":"1.0.5"}]}}`
	verify := `{"code":1,"data":{"has_permission":false,"link":"","msg":"ip未授权"}}`
	if err := os.WriteFile(filepath.Join(fixture, "lists.json"), []byte(lists), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture, "verify.json"), []byte(verify), 0644); err != nil {
		t.Fatal(err)
	}
	tree := t.TempDir()
	public := filepath.Join(tree, "server", "public")
	if err := os.MkdirAll(filepath.Join(tree, "server", "upgrade"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tree, "server", "upgrade", "version.json"), []byte(`{"version":"1.0.5"}`), 0644); err != nil {
		t.Fatal(err)
	}
	oldPub := config.C.App.PublicDir
	t.Cleanup(func() { config.C.App.PublicDir = oldPub })
	config.C.App.PublicDir = public
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", fixture)
	cache.Del("version_lists")

	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]any{"id": 99, "update_type": 1})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/upgrade.upgrade/upgrade", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Host = "pair1.likeadmin.test"
	UpgradeDo(c)

	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code == 1 || wrap.Msg != "更新失败:ip未授权" {
		t.Fatalf("denied %+v", wrap)
	}
}
