package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRestoreIndexFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("lock-page"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index_lock.html"), []byte("spa"), 0644); err != nil {
		t.Fatal(err)
	}
	restoreIndexFile(dir)
	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "spa" {
		t.Fatalf("got %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "index_lock.html")); err == nil {
		t.Fatal("lock file should be gone")
	}
}

func TestProbeDir(t *testing.T) {
	item := probeDir("runtime", t.TempDir())
	if item.Status != "ok" {
		t.Fatalf("%+v", item)
	}
	item = probeDir("missing", "")
	if item.Status != "fail" {
		t.Fatal(item)
	}
}

func TestProbeWritableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("x=1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	item := probeWritableFile(".env", path)
	if item.Status != "ok" {
		t.Fatalf("%+v", item)
	}
	missing := probeWritableFile(".env", filepath.Join(dir, "no-such.env"))
	if missing.Status != "fail" || missing.Value != "文件不存在" {
		t.Fatalf("%+v", missing)
	}
	empty := probeWritableFile(".env", "")
	if empty.Status != "fail" || empty.Value != "未配置" {
		t.Fatalf("%+v", empty)
	}
}

func TestPublicSub(t *testing.T) {
	if got := publicSub("uploads"); !strings.HasSuffix(got, "uploads") {
		t.Fatalf("got %s", got)
	}
}
