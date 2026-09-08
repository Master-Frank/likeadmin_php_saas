package upgrade

import (
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestGetRemoteVersionSkipsEmptyCache(t *testing.T) {
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", "")
	cache.Del("version_lists1")
	cache.Set("version_lists1", `{"count":0,"lists":[]}`, time.Hour)
	dir := t.TempDir()
	body := `{"code":1,"data":{"count":1,"lists":[{"id":9,"version_no":"9.9.9"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", dir)
	payload := GetRemoteVersion(1, 25)
	rows, _ := payload["lists"].([]any)
	if len(rows) != 1 {
		t.Fatalf("should refetch after empty cache: %+v", payload)
	}
}

func TestGetRemoteVersionEmptyBodyIsNotVersionMiss(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", dir)
	cache.Del("version_lists")
	payload := GetRemoteVersion(0, 0)
	if hasVersionLists(payload) {
		t.Fatalf("empty body should not invent lists: %+v", payload)
	}
	// Empty remote lists stay empty; the miss copy belongs to VersionByID / download.
	if CheckVersionData(2) != "未获取到对应版本信息" {
		t.Fatal("VersionByID miss must stay 未获取到对应版本信息")
	}
	if checkAbleUpgrade("1.0.5", VersionByID(2), payload) != "未获取到对应版本信息" {
		t.Fatal("empty target is version miss, not lists failure")
	}

	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte(`{"code":1,"data":{"count":0,"lists":[]}}`), 0644); err != nil {
		t.Fatal(err)
	}
	cache.Del("version_lists")
	emptyLists := GetRemoteVersion(0, 0)
	if hasVersionLists(emptyLists) {
		t.Fatalf("empty lists payload: %+v", emptyLists)
	}
	if CheckVersionData(2) != "未获取到对应版本信息" {
		t.Fatal("empty lists is still a VersionByID miss")
	}
}

func TestFetchJSONNonJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", dir)
	cache.Del("version_lists")
	if wrap := fetchJSON("http://upgrade.local/indexapi/version/lists?action=lists"); wrap != nil {
		t.Fatalf("non-json must be nil: %+v", wrap)
	}
	if payload := GetRemoteVersion(0, 0); hasVersionLists(payload) || payload == nil {
		t.Fatalf("non-json lists: %+v", payload)
	}
}

func TestCheckVersionDataMiss(t *testing.T) {
	dir := t.TempDir()
	body := `{"code":1,"data":{"count":1,"lists":[{"id":2,"version_no":"1.0.6"}]}}`
	if err := os.WriteFile(filepath.Join(dir, "lists.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_UPGRADE_FIXTURE", dir)
	cache.Del("version_lists")
	if CheckVersionData(2) != "" {
		t.Fatal(CheckVersionData(2))
	}
	if CheckVersionData(99) != "未获取到对应版本信息" {
		t.Fatal(CheckVersionData(99))
	}
	if CheckVersionData(0) != "未获取到对应版本信息" {
		t.Fatal("id 0")
	}
}

func TestParseJSONBody(t *testing.T) {
	if parseJSONBody(nil) != nil || parseJSONBody([]byte("")) != nil || parseJSONBody([]byte("not-json")) != nil {
		t.Fatal("empty/non-json")
	}
	got := parseJSONBody([]byte(`{"code":1,"data":{}}`))
	if got == nil || got["code"] != float64(1) {
		t.Fatalf("%+v", got)
	}
}

func TestHasVersionLists(t *testing.T) {
	if hasVersionLists(nil) || hasVersionLists(map[string]any{}) || hasVersionLists(map[string]any{"lists": []any{}}) {
		t.Fatal("empty payload treated as ready")
	}
	if !hasVersionLists(map[string]any{"lists": []any{map[string]any{"id": 1}}}) {
		t.Fatal("non-empty lists rejected")
	}
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
