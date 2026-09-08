package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"likeadmin/backend/internal/config"
)

func TestInitAllowsMissingDBBeforeInstall(t *testing.T) {
	oldDB, oldC, oldPath := DB, config.C, config.Path
	t.Cleanup(func() {
		DB = oldDB
		config.C = oldC
		config.Path = oldPath
	})

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "public"), 0755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "config.yaml")
	body := []byte("app:\n  debug: false\n  timezone: Asia/Shanghai\n  public_dir: " + filepath.Join(dir, "public") + "\n  install_lock: " + filepath.Join(dir, "config", "install.lock") + "\ndatabase:\n  hostname: 127.0.0.1\n  hostport: 3306\n  database: likeadmin_missing_db_xyz\n  username: likeadmin\n  password: root\n  charset: utf8mb4\n  prefix: la_\nredis:\n  host: 127.0.0.1\n  port: 6379\n  prefix: \"la:\"\n")
	if err := os.WriteFile(cfg, body, 0644); err != nil {
		t.Fatal(err)
	}
	if err := Init(cfg); err != nil {
		t.Fatal(err)
	}
	if Installed() {
		t.Fatal("lock must be absent")
	}
	if DB != nil {
		t.Fatal("DB should stay nil before install")
	}
}

func TestInstalled(t *testing.T) {
	old := config.C.App.InstallLock
	t.Cleanup(func() { config.C.App.InstallLock = old })
	config.C.App.InstallLock = ""
	if Installed() {
		t.Fatal("empty lock")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "install.lock")
	config.C.App.InstallLock = path
	if Installed() {
		t.Fatal("missing file")
	}
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	if !Installed() {
		t.Fatal("present file")
	}
}
