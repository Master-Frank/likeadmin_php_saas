package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/cache"
)

func TestFixtureListsAndVerify(t *testing.T) {
	dir := t.TempDir()
	lists := `{"code":1,"data":{"count":2,"lists":[{"id":2,"version_no":"1.0.6"},{"id":1,"version_no":"1.0.5"}]}}`
	verify := `{"code":1,"data":{"has_permission":true,"link":"file:///tmp/likeadmin-offline.zip","msg":""}}`
	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte(lists), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verify.json"), []byte(verify), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", dir)
	cache.Del("version_lists")
	cache.Del("version_lists1")

	payload := GetRemoteVersion(0, 0)
	rows, _ := payload["lists"].([]any)
	if payload["count"] == nil || len(rows) != 2 {
		t.Fatalf("lists %+v", payload)
	}
	if VersionByID(2)["version_no"] != "1.0.6" {
		t.Fatalf("by id %+v", VersionByID(2))
	}
	got := Verify("pair1.likeadmin.test", 2, "package_link")
	if !HasPermission(got) || got["link"] != "file:///tmp/likeadmin-offline.zip" {
		t.Fatalf("verify %+v", got)
	}
	AddLog("pair1.likeadmin.test", 2, 1, true, "")
}

func TestUpgradeBaseOverride(t *testing.T) {
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", "")
	t.Setenv("LIKEADMIN_UPGRADE_BASE_URL", "http://upgrade.local/")
	if upgradeBase() != "http://upgrade.local" {
		t.Fatal(upgradeBase())
	}
	t.Setenv("LIKEADMIN_UPGRADE_BASE_URL", "")
	if upgradeBase() != BaseURL {
		t.Fatal(upgradeBase())
	}
}
