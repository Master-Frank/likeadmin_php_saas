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
	if normalizeCommand("version") != "version" {
		t.Fatal(normalizeCommand("version"))
	}
	if normalizeCommand("optimize:schema") != "optimize:schema" {
		t.Fatal(normalizeCommand("optimize:schema"))
	}
	if normalizeCommand("route:list") != "route:list" {
		t.Fatal(normalizeCommand("route:list"))
	}
}

func TestRunCommandUnknown(t *testing.T) {
	got := runCommand(model.Crontab{Command: "not_a_real_command"})
	if got != "未定义的定时任务命令: not_a_real_command" {
		t.Fatalf("got %q", got)
	}
}

func TestCrontabFinishUpdatesLastTimeAfterRun(t *testing.T) {
	start := time.Now().Add(-1500 * time.Millisecond)
	got := crontabFinishUpdates(model.Crontab{MaxTime: "0.10"}, start, "")
	last, _ := got["last_time"].(int64)
	if last < time.Now().Unix()-1 {
		t.Fatalf("last_time=%v should be now, not loop-start", got["last_time"])
	}
	if got["error"] != "" || got["status"] != nil {
		t.Fatalf("ok run %+v", got)
	}
	fail := crontabFinishUpdates(model.Crontab{}, time.Now(), "boom")
	if fail["error"] != "boom" || fail["status"] != 3 {
		t.Fatalf("err run %+v", fail)
	}
}

func TestEnsureNativeJobsNilDB(t *testing.T) {
	EnsureNativeJobs()
}

func TestCommandRegistry(t *testing.T) {
	names := CommandNames()
	want := map[string]bool{
		"cache": true, "clear": true, "session": true, "query_refund": true,
		"cancel_unpaid_orders": true, "version": true, "optimize:schema": true,
	}
	for _, n := range names {
		delete(want, n)
	}
	if len(want) > 0 {
		t.Fatalf("missing builtins %v in %v", want, names)
	}
	Register("warehouse_sync", func(args []string) string {
		if len(args) == 1 && args[0] == "--once" {
			return ""
		}
		return "bad-args"
	})
	t.Cleanup(func() { Unregister("warehouse_sync") })
	if got := runCommand(model.Crontab{Command: "warehouse_sync", Params: "--once"}); got != "" {
		t.Fatalf("registered command: %q", got)
	}
	if got := RunNamed("warehouse_sync", "--once"); got != "" {
		t.Fatalf("RunNamed registered: %q", got)
	}
	found := false
	for _, n := range CommandNames() {
		if n == "warehouse_sync" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("CommandNames missing warehouse_sync")
	}
	Unregister("warehouse_sync")
	if got := runCommand(model.Crontab{Command: "warehouse_sync"}); got != "未定义的定时任务命令: warehouse_sync" {
		t.Fatalf("unregistered: %q", got)
	}
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
	if err := os.MkdirAll(filepath.Join(dir, "runtime", "platformapi", "cache"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime", "platformapi", "cache", "x.php"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime", "extra.dat"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime", ".gitignore"), []byte("*\n"), 0644); err != nil {
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
	if _, err := os.Stat(filepath.Join(dir, "runtime", "extra.dat")); err == nil {
		t.Fatal("runtime extra files should be cleared")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "platformapi", "cache", "x.php")); err == nil {
		t.Fatal("runtime cache files should be cleared")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "curd-keep.zip")); err != nil {
		t.Fatal("generator zip should stay")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", ".gitignore")); err != nil {
		t.Fatal(".gitignore should stay")
	}
}

func TestParseClearArgs(t *testing.T) {
	o := parseClearArgs([]string{"--cache", "--expire", "--dir"})
	if !o.cache || !o.expire || !o.rmdir || o.log || o.path != "" {
		t.Fatalf("%+v", o)
	}
	o = parseClearArgs([]string{"-d", "runtime/cache", "-r"})
	if o.path != "runtime/cache" || !o.rmdir || o.cache {
		t.Fatalf("%+v", o)
	}
	o = parseClearArgs([]string{"--path=/tmp/outside"})
	if o.path != "/tmp/outside" {
		t.Fatalf("%+v", o)
	}
}

func TestClearPathRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	defer func() { config.C.App.PublicDir = old }()
	if msg := runClear([]string{"--path", "/etc"}); msg != "清理路径不合法" {
		t.Fatalf("escape %q", msg)
	}
}

func TestClearExpireAndRmdir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "runtime", "cache", "la")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(cacheDir, "fresh.php")
	expired := filepath.Join(cacheDir, "old.php")
	if err := os.WriteFile(fresh, []byte("<?php\n//000000360000payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(expired, []byte("<?php\n//000000000001payload"), 0644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-10 * time.Second)
	if err := os.Chtimes(expired, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	defer func() { config.C.App.PublicDir = old }()
	if msg := runClear([]string{"--cache", "--expire", "--dir"}); msg != "" {
		t.Fatal(msg)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatal("unexpired cache should stay")
	}
	if _, err := os.Stat(expired); err == nil {
		t.Fatal("expired cache should be removed")
	}
}

func TestRunCommandWithParams(t *testing.T) {
	if got := runCommand(model.Crontab{Command: "cache", Params: "--unused extra"}); got != "" {
		t.Fatalf("cache with params: %q", got)
	}
	if got := runCommand(model.Crontab{Command: "not_a_real_command", Params: "foo"}); got != "未定义的定时任务命令: not_a_real_command" {
		t.Fatalf("unknown with params: %q", got)
	}
}

func TestClearRuntimeCacheFlag(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "runtime", "cache")
	logDir := filepath.Join(dir, "runtime", "log")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "x"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "y"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runtime", "extra.dat"), []byte("1"), 0644); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	defer func() { config.C.App.PublicDir = old }()
	if msg := runClear([]string{"--cache"}); msg != "" {
		t.Fatal(msg)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "x")); err == nil {
		t.Fatal("runtime/cache should be cleared")
	}
	if _, err := os.Stat(filepath.Join(logDir, "y")); err != nil {
		t.Fatal("runtime/log should stay for --cache")
	}
	if _, err := os.Stat(filepath.Join(dir, "runtime", "extra.dat")); err != nil {
		t.Fatal("runtime extra should stay for --cache")
	}
}
