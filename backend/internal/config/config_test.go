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

func TestReplicaListSkipsEmptyHost(t *testing.T) {
	d := DatabaseConfig{Replicas: []DatabaseConfig{{Hostname: ""}, {Hostname: "10.0.0.2"}}}
	got := d.ReplicaList()
	if len(got) != 1 || got[0].Hostname != "10.0.0.2" {
		t.Fatalf("%+v", got)
	}
}

func TestExportAsyncAndRedisFlags(t *testing.T) {
	oldApp, oldProj := C.App, C.Project
	t.Cleanup(func() {
		C.App, C.Project = oldApp, oldProj
	})
	C.App.MultiInstance = false
	C.App.RequireRedis = false
	C.Project.ExportAsync = false
	t.Setenv("LIKEADMIN_REQUIRE_REDIS", "")
	t.Setenv("LIKEADMIN_EXPORT_ASYNC", "")
	if RequireRedisConfigured() || ExportAsyncEnabled() {
		t.Fatal("single-node defaults must not force redis/async export")
	}
	C.App.MultiInstance = true
	if !RequireRedisConfigured() || !ExportAsyncEnabled() {
		t.Fatal("multi-instance must require redis and async export")
	}
}

func TestFileCDNDomain(t *testing.T) {
	old := C.App.CDNDomain
	t.Cleanup(func() { C.App.CDNDomain = old })
	C.App.CDNDomain = ""
	if FileCDNDomain() != "" {
		t.Fatal("empty cdn")
	}
	C.App.CDNDomain = "cdn.example.com/"
	if FileCDNDomain() != "https://cdn.example.com" {
		t.Fatalf("%s", FileCDNDomain())
	}
	C.App.CDNDomain = "http://cdn.local"
	if FileCDNDomain() != "http://cdn.local" {
		t.Fatalf("%s", FileCDNDomain())
	}
}
