package cron

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
)

func TestThinkListAndHelp(t *testing.T) {
	raw := formatCommandList(true, "")
	for _, name := range []string{"cache", "clear", "help", "list", "crontab", "version"} {
		found := false
		for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
			if line == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("raw list missing %s: %s", name, raw)
		}
	}
	if !strings.Contains(formatCommandList(false, ""), "Clear runtime file") {
		t.Fatal("formatted list should include descriptions")
	}
	opt := formatCommandList(true, "optimize")
	if !strings.Contains(opt, "optimize:schema") {
		t.Fatalf("namespace filter %s", opt)
	}
	for _, line := range strings.Split(strings.TrimSpace(opt), "\n") {
		if line == "clear" {
			t.Fatalf("namespace filter leaked clear: %s", opt)
		}
	}
	if got := RunNamed("help", "clear"); got != "" {
		t.Fatalf("help clear: %q", got)
	}
	if got := RunNamed("help", "not_a_real_command"); got != "未定义的命令: not_a_real_command" {
		t.Fatalf("help unknown: %q", got)
	}
	if got := RunNamed("list", "--raw"); got != "" {
		t.Fatalf("list --raw: %q", got)
	}
}

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
