package upgrade

import (
	"archive/zip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
)

func writeZip(t *testing.T, entries map[string]string) string {
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

func TestResolvePackageLocalAndHTTP(t *testing.T) {
	local := writeZip(t, map[string]string{"project/server/probe.txt": "ok"})
	got, err := resolvePackage(local, t.TempDir())
	if err != nil || got != local {
		t.Fatalf("local: %s %v", got, err)
	}
	got, err = resolvePackage("file://"+local, t.TempDir())
	if err != nil || got != local {
		t.Fatalf("file: %s %v", got, err)
	}
	if _, err := resolvePackage("file://"+filepath.Join(t.TempDir(), "missing.zip"), t.TempDir()); err == nil {
		t.Fatal("missing file:// should fail")
	}

	body, err := os.ReadFile(local)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/pkg.zip" {
			http.NotFound(w, r)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	saveDir := t.TempDir()
	got, err = resolvePackage(srv.URL+"/pkg.zip", saveDir)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(got) != "pkg.zip" {
		t.Fatalf("downloaded name %s", got)
	}
	if raw, err := os.ReadFile(got); err != nil || string(raw) != string(body) {
		t.Fatalf("downloaded bytes mismatch err=%v", err)
	}
	if _, err := resolvePackage(srv.URL+"/missing.zip", t.TempDir()); err == nil {
		t.Fatal("http 404 should fail")
	}
}

func TestApplyExtractedCopiesServerAndBackend(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "project", "server", "public"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "project", "backend", "internal", "pkg"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "project", "server", "public", "probe.txt"), []byte("front"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "project", "backend", "internal", "pkg", "x.go"), []byte("package pkg\n"), 0644); err != nil {
		t.Fatal(err)
	}
	serverDest := t.TempDir()
	backendDest := t.TempDir()
	if err := applyExtracted(src, serverDest, backendDest, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(serverDest, "public", "probe.txt")); err != nil || string(got) != "front" {
		t.Fatalf("server file: %q %v", got, err)
	}
	if got, err := os.ReadFile(filepath.Join(backendDest, "internal", "pkg", "x.go")); err != nil || string(got) != "package pkg\n" {
		t.Fatalf("backend file: %q %v", got, err)
	}
}

func TestApplyExtractedCopiesNewPublicPrefix(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "project", "public"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "project", "public", "probe.txt"), []byte("new-layout"), 0644); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	if err := applyExtracted(src, dest, t.TempDir(), nil); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "public", "probe.txt"))
	if err != nil || string(got) != "new-layout" {
		t.Fatalf("new public prefix: %q %v", got, err)
	}
}

func TestApplyLocalWritesVersionJSON(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"project/server/public/local-probe.txt": "apply-local",
	})
	tree := t.TempDir()
	public := filepath.Join(tree, "proj", "public")
	if err := os.MkdirAll(public, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tree, "proj", "upgrade"), 0755); err != nil {
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

	if err := ApplyLocal(zipPath, "2.1.0"); err != nil {
		t.Fatal(err)
	}
	if LocalVersion() != "2.1.0" {
		t.Fatalf("version %s", LocalVersion())
	}
	got, err := os.ReadFile(filepath.Join(tree, "proj", "public", "local-probe.txt"))
	if err != nil || string(got) != "apply-local" {
		t.Fatalf("probe %q err=%v", got, err)
	}
}

func TestApplyLocalUnzipsThenCopies(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"project/server/public/probe.txt":      "from-zip",
		"project/backend/internal/pkg/x.go":    "package pkg\n",
		"project/sql/data/skip-without-db.sql": "SELECT 1;",
	})
	extract := t.TempDir()
	if err := unzip(zipPath, extract); err != nil {
		t.Fatal(err)
	}
	serverDest := t.TempDir()
	backendDest := t.TempDir()
	if err := applyExtracted(extract, serverDest, backendDest, nil); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(filepath.Join(serverDest, "public", "probe.txt")); err != nil || string(got) != "from-zip" {
		t.Fatalf("unzip+apply server: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(backendDest, "internal", "pkg", "x.go")); err != nil {
		t.Fatalf("unzip+apply backend: %v", err)
	}
}

func TestUnzipSkipsParentPaths(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"../escape.txt":           "bad",
		"project/server/safe.txt": "ok",
	})
	dest := t.TempDir()
	if err := unzip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "escape.txt")); err == nil {
		t.Fatal("parent path should be skipped")
	}
	if got, err := os.ReadFile(filepath.Join(dest, "project", "server", "safe.txt")); err != nil || string(got) != "ok" {
		t.Fatalf("safe=%q %v", got, err)
	}
}

func TestVersionFromFilename(t *testing.T) {
	if got := versionFromFilename("/tmp/likeadmin-1.8.0.zip"); got != "1.8.0" {
		t.Fatalf("%s", got)
	}
	if got := versionFromFilename("pkg.zip"); got != "" {
		t.Fatalf("empty %q", got)
	}
}

func TestResolvePackageRejectsRedirect(t *testing.T) {
	// PHP curl downFile does not FOLLOWLOCATION; a 302 HTML page must not be saved.
	html := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>login</html>"))
	}))
	t.Cleanup(html.Close)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, html.URL+"/trap.zip", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	if _, err := resolvePackage(srv.URL+"/pkg.zip", t.TempDir()); err == nil || !strings.Contains(err.Error(), "获取文件错误") {
		t.Fatalf("redirect: %v", err)
	}
}

func TestResolvePackageEmptyHTTPBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if _, err := resolvePackage(srv.URL+"/empty.zip", t.TempDir()); err == nil || !strings.Contains(err.Error(), "获取文件错误") {
		t.Fatalf("empty 200: %v", err)
	}
}

func TestApplyExtractedSQLDataAndStructure(t *testing.T) {
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if bootstrap.DB == nil {
		if err := bootstrap.Init(cfg); err != nil {
			t.Skip(err)
		}
	}
	if bootstrap.DB == nil {
		t.Skip("no database")
	}
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "project", "sql", "data"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "project", "sql", "structure"), 0755); err != nil {
		t.Fatal(err)
	}
	dataSQL := "CREATE TABLE IF NOT EXISTS `la_pair_upgrade_probe` (`id` int NOT NULL);\nINSERT INTO `la_pair_upgrade_probe` (`id`) VALUES (46);"
	structSQL := "CREATE TABLE IF NOT EXISTS `la_pair_upgrade_probe_s` (`id` int NOT NULL);"
	if err := os.WriteFile(filepath.Join(src, "project", "sql", "data", "001.sql"), []byte(dataSQL), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "project", "sql", "structure", "001.sql"), []byte(structSQL), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = bootstrap.DB.Exec("DROP TABLE IF EXISTS la_pair_upgrade_probe").Error
		_ = bootstrap.DB.Exec("DROP TABLE IF EXISTS la_pair_upgrade_probe_s").Error
	})
	if err := applyExtracted(src, t.TempDir(), t.TempDir(), bootstrap.DB); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := bootstrap.DB.Raw("SELECT COUNT(*) FROM la_pair_upgrade_probe WHERE id = 46").Scan(&n).Error; err != nil || n != 1 {
		t.Fatalf("data sql n=%d err=%v", n, err)
	}
	if err := bootstrap.DB.Raw("SELECT COUNT(*) FROM la_pair_upgrade_probe_s").Scan(&n).Error; err != nil {
		t.Fatalf("structure sql: %v", err)
	}
}

func TestListUpgradeTenantsSkipsDeleted(t *testing.T) {
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if bootstrap.DB == nil {
		if err := bootstrap.Init(cfg); err != nil {
			t.Skip(err)
		}
	}
	if bootstrap.DB == nil {
		t.Skip("no database")
	}
	now := util.NowUnix()
	row := model.Tenant{
		SN: "upgdel61", Name: "upgrade-deleted", Tactics: 0,
		CreateTime: now, UpdateTime: &now, DeleteTime: &now,
	}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Unscoped().Delete(&row) })
	for _, tnt := range listUpgradeTenants(bootstrap.DB) {
		if tnt.ID == row.ID {
			t.Fatal("PHP Tenant SoftDelete must skip deleted tenants on upgradeMenu")
		}
		if tnt.DeleteTime != nil {
			t.Fatalf("live tenant %d has delete_time", tnt.ID)
		}
	}
}

func TestResolvePackageEmpty(t *testing.T) {
	if _, err := resolvePackage("", t.TempDir()); err == nil || !strings.Contains(err.Error(), "获取文件错误") {
		t.Fatalf("empty: %v", err)
	}
}

func TestDownFileRejectsHTML200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><html>license denied</html>"))
	}))
	t.Cleanup(srv.Close)
	if _, err := resolvePackage(srv.URL+"/pkg.zip", t.TempDir()); err == nil || !strings.Contains(err.Error(), "获取文件错误") {
		t.Fatalf("html 200: %v", err)
	}
}

func TestDownFileRejectsSelfSignedTLS(t *testing.T) {
	t.Setenv("LIKEADMIN_UPGRADE_INSECURE_TLS", "")
	body, err := os.ReadFile(writeZip(t, map[string]string{"project/server/ok.txt": "tls"}))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	if _, err := resolvePackage(srv.URL+"/pkg.zip", t.TempDir()); err == nil {
		t.Fatal("self-signed TLS must fail unless LIKEADMIN_UPGRADE_INSECURE_TLS=1")
	}
}

func TestDownFileAcceptsSelfSignedTLS(t *testing.T) {
	t.Setenv("LIKEADMIN_UPGRADE_INSECURE_TLS", "1")
	body, err := os.ReadFile(writeZip(t, map[string]string{"project/server/ok.txt": "tls"}))
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	got, err := resolvePackage(srv.URL+"/pkg.zip", t.TempDir())
	if err != nil {
		t.Fatalf("LIKEADMIN_UPGRADE_INSECURE_TLS=1 should match PHP CURLOPT_SSL_VERIFYPEER=false: %v", err)
	}
	raw, err := os.ReadFile(got)
	if err != nil || string(raw) != string(body) {
		t.Fatalf("tls bytes mismatch err=%v", err)
	}
}

func TestIsZipMagic(t *testing.T) {
	if isZipMagic([]byte("PK\x03\x04")) && isZipMagic([]byte("PK\x05\x06")) && isZipMagic([]byte("PK\x07\x08")) {
		if isZipMagic([]byte("<htm")) || isZipMagic([]byte("PK")) || isZipMagic(nil) {
			t.Fatal("non-zip accepted")
		}
		return
	}
	t.Fatal("zip magic rejected")
}
