package cron

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
)

func TestNormalizeCommand(t *testing.T) {
	if normalizeCommand(`app\common\command\QueryRefund`) != "query_refund" {
		t.Fatal(normalizeCommand(`app\common\command\QueryRefund`))
	}
	if normalizeCommand("crontab") != "crontab" {
		t.Fatal(normalizeCommand("crontab"))
	}
	if normalizeCommand("clear_session") != "session" {
		t.Fatal(normalizeCommand("clear_session"))
	}
	if normalizeCommand("clear") != "clear" {
		t.Fatal(normalizeCommand("clear"))
	}
}

func TestRunCommandUnknown(t *testing.T) {
	got := runCommand(model.Crontab{Command: "not_a_real_command"})
	if got != "未定义的定时任务命令: not_a_real_command" {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureNativeJobsNilDB(t *testing.T) {
	EnsureNativeJobs()
}

func TestRunNamed(t *testing.T) {
	if RunNamed("not_a_real_command") != "未定义的定时任务命令: not_a_real_command" {
		t.Fatal(RunNamed("not_a_real_command"))
	}
	if got := RunNamed(`app\common\command\QueryRefund`); len(got) >= 3 && got[:3] == "未定" {
		t.Fatalf("native query_refund should not fall through, got %q", got)
	}
	if got := RunNamed("crontab"); got != "" {
		t.Fatalf("think crontab is RunOnce, got %q", got)
	}
	if got := runCommand(model.Crontab{Command: "crontab"}); got != "未定义的定时任务命令: crontab" {
		t.Fatalf("db row crontab must not recurse: %q", got)
	}
}

func TestDropSessionTokens(t *testing.T) {
	cache.Set("token_admin_dead", map[string]any{"token": "dead"}, time.Hour)
	cache.Set("token_tenant_dead", map[string]any{"token": "dead"}, time.Hour)
	cache.Set("token_user_dead", map[string]any{"token": "dead"}, time.Hour)
	dropSessionTokens(
		[]model.AdminSession{{Token: "dead"}},
		[]model.TenantAdminSession{{Token: "dead"}},
		[]model.UserSession{{Token: "dead"}},
	)
	if _, ok := cache.Get("token_admin_dead"); ok {
		t.Fatal("admin token cache should drop")
	}
	if _, ok := cache.Get("token_tenant_dead"); ok {
		t.Fatal("tenant token cache should drop")
	}
	if _, ok := cache.Get("token_user_dead"); ok {
		t.Fatal("user token cache should drop")
	}
}

func TestClearRuntimeWipesFile(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "public")
	fileDir := filepath.Join(dir, "runtime", "file")
	cacheDir := filepath.Join(dir, "runtime", "cache")
	if err := os.MkdirAll(fileDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fileDir, "x"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "y"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime", "curd-keep.zip"), []byte("z"), 0644); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.PublicDir
	config.C.App.PublicDir = pub
	defer func() { config.C.App.PublicDir = old }()
	if msg := clearRuntime(); msg != "" {
		t.Fatal(msg)
	}
	if _, err := os.Stat(filepath.Join(fileDir, "x")); err == nil {
		t.Fatal("runtime/file should be cleared")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "curd-keep.zip")); err != nil {
		t.Fatal("generator zip should stay")
	}
}
