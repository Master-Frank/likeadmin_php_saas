package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLikeadminDebugOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: true\n  public_dir: public\n  install_lock: config/install.lock\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_DEBUG", "false")
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if C.App.Debug {
		t.Fatal("LIKEADMIN_DEBUG=false must override the development config")
	}
}
