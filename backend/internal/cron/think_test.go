package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
)

func TestRunVersion(t *testing.T) {
	if got := RunNamed("version"); got != "" {
		t.Fatalf("version: %q", got)
	}
}

func TestOptimizeSchemaWritesCache(t *testing.T) {
	if bootstrap.DB == nil {
		cfg := os.Getenv("LIKEADMIN_CONFIG")
		if cfg == "" {
			cfg = "/workspace/backend/configs/config.yaml"
		}
		if err := bootstrap.Init(cfg); err != nil {
			t.Skip(err)
		}
	}
	if bootstrap.DB == nil {
		t.Skip("no database")
	}
	if got := RunNamed("optimize:schema", "--table", "la_config"); got != "" {
		t.Fatal(got)
	}
	dbName := config.C.Database.Database
	if dbName == "" {
		dbName = "likeadmin_saas"
	}
	path := filepath.Join(filepath.Dir(config.C.App.PublicDir), "runtime", "schema", dbName+".la_config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"name":"id"`) || !strings.Contains(string(b), `"name":"type"`) {
		t.Fatalf("schema cache %s", b)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}
