package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
)

func TestApplyAuthorizedFixtureZip(t *testing.T) {
	zipPath := writeZip(t, map[string]string{
		"project/server/public/upgrade-probe.txt": "authorized-ok",
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
	cache.Del("version_lists")
	cache.Del("version_lists1")

	if msg := CheckAbleUpgrade(99); msg != "" {
		t.Fatalf("able upgrade: %s", msg)
	}
	if err := ApplyAuthorized("pair1.likeadmin.test", 99); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(tree, "proj", "public", "upgrade-probe.txt")
	got, err := os.ReadFile(probe)
	if err != nil || string(got) != "authorized-ok" {
		t.Fatalf("probe %q err=%v", got, err)
	}
	if LocalVersion() != "9.9.9" {
		t.Fatalf("version %s", LocalVersion())
	}
	if _, err := os.Stat("/workspace/server/public/upgrade-probe.txt"); err == nil {
		t.Fatal("must not write probe into pairing server tree")
	}
}

func TestApplyAuthorizedDenied(t *testing.T) {
	dir := t.TempDir()
	lists := `{"code":1,"data":{"count":2,"lists":[{"id":99,"version_no":"9.9.9"},{"id":1,"version_no":"1.0.5"}]}}`
	verify := `{"code":1,"data":{"has_permission":false,"link":"","msg":"ip未授权"}}`
	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte(lists), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verify.json"), []byte(verify), 0644); err != nil {
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
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", dir)
	cache.Del("version_lists")
	err := ApplyAuthorized("pair1.likeadmin.test", 99)
	if err == nil || err.Error() != "ip未授权" {
		t.Fatalf("denied: %v", err)
	}
}
