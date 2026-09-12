package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
)

func TestWriteGoConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: true\ndatabase:\n  hostname: old\n  prefix: la_\nproject:\n  unique_identification: likeadmin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldDB, oldPrefix, oldSalt := config.C.Database, config.C.Database.Prefix, config.C.Project.UniqueIdentification
	defer func() {
		config.C.Database = oldDB
		config.C.Database.Prefix = oldPrefix
		config.C.Project.UniqueIdentification = oldSalt
	}()
	if err := WriteGoConfig(path, "10.0.0.8", "newdb", "u", "p", 3307, "xx_", "host.test", "abcd"); err != nil {
		t.Fatal(err)
	}
	if config.C.Database.Hostname != "10.0.0.8" || config.C.Database.Prefix != "xx_" || config.C.Project.UniqueIdentification != "abcd" {
		t.Fatalf("memory %+v salt=%s", config.C.Database, config.C.Project.UniqueIdentification)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"10.0.0.8", "newdb", "xx_", "abcd", "3307"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
	if config.C.App.MultiInstance || strings.Contains(s, "hostname: 10.0.0.9") {
		t.Fatal("single-node write must not enable multi-instance or replica")
	}
}

func TestWriteGoConfigOptsReplicaAndMulti(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: true\ndatabase:\n  hostname: old\nredis:\n  host: 127.0.0.1\nproject:\n  unique_identification: likeadmin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldApp, oldDB, oldProj, oldRedis := config.C.App, config.C.Database, config.C.Project, config.C.Redis
	defer func() {
		config.C.App, config.C.Database, config.C.Project, config.C.Redis = oldApp, oldDB, oldProj, oldRedis
	}()
	if err := WriteGoConfigOpts(GoWrite{
		Path: path, Host: "10.0.0.8", DBName: "newdb", User: "u", Pass: "p", Port: 3306,
		Prefix: "la_", UniqueID: "salt", MultiInstance: true, ExportAsync: true, RequireRedis: true,
		RedisHost: "10.0.0.7", RedisPort: 6379, ReplicaHost: "10.0.0.9", ReplicaPort: 3306,
		CDNDomain: "https://cdn.example.com",
	}); err != nil {
		t.Fatal(err)
	}
	if !config.C.App.MultiInstance || !config.C.Project.ExportAsync || config.C.Redis.Host != "10.0.0.7" {
		t.Fatalf("memory multi %+v redis=%s", config.C.App, config.C.Redis.Host)
	}
	if len(config.C.Database.Replicas) != 1 || config.C.Database.Replicas[0].Hostname != "10.0.0.9" {
		t.Fatalf("replicas %+v", config.C.Database.Replicas)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"10.0.0.9", "10.0.0.7", "cdn.example.com", "export_async"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
}
