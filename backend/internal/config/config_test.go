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

func TestPoolAndExportDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: false\n  public_dir: public\n  install_lock: lock\ndatabase:\n  hostname: 127.0.0.1\nredis:\n  host: 127.0.0.1\nproject:\n  lists:\n    page_size_max: 25000\n    page_size: 25\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_DEBUG", "")
	t.Setenv("LIKEADMIN_DB_MAX_OPEN", "80")
	t.Setenv("LIKEADMIN_DB_MAX_IDLE", "20")
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
	if C.Database.MaxOpenConns != 80 || C.Database.MaxIdleConns != 20 {
		t.Fatalf("pool env %+v", C.Database)
	}
	if C.Database.ConnMaxLifetime != 300 || C.Database.ConnMaxIdleTime != 60 {
		t.Fatalf("lifetime %+v", C.Database)
	}
	if C.Project.Lists.ExportMaxRows != 10000 || C.Project.Lists.ExportMaxPages != 20 {
		t.Fatalf("export caps %+v", C.Project.Lists)
	}
	if C.Redis.ReadTimeoutMs != 200 {
		t.Fatalf("redis timeout %d", C.Redis.ReadTimeoutMs)
	}
}
