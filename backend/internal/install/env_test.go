package install

import (
	"os"
	"path/filepath"
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
