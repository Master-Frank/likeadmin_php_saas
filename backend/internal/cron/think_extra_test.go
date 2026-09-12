package cron

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
)

func TestParseRunArgs(t *testing.T) {
	got := ParseRunArgs(nil)
	if got.Host != "0.0.0.0" || got.Port != "8000" || got.Addr() != "0.0.0.0:8000" {
		t.Fatalf("%+v", got)
	}
	got = ParseRunArgs([]string{"--host", "127.0.0.1", "--port", "8090", "--root", "/tmp/pub"})
	if got.Host != "127.0.0.1" || got.Port != "8090" || got.Root != "/tmp/pub" {
		t.Fatalf("%+v", got)
	}
	got = ParseRunArgs([]string{"-H=0.0.0.0", "-p=9000", "-r=/var/www"})
	if got.Host != "0.0.0.0" || got.Port != "9000" || got.Root != "/var/www" {
		t.Fatalf("%+v", got)
	}
}

func TestPHPScaffoldDisabledByDefault(t *testing.T) {
	t.Setenv("LIKEADMIN_ENABLE_PHP_SCAFFOLD", "")
	if got := RunNamed("make:model", "Demo"); got != legacyPHPDisabled {
		t.Fatalf("make:model=%q", got)
	}
	if got := RunNamed("build", "demo"); got != legacyPHPDisabled {
		t.Fatalf("build=%q", got)
	}
	if got := RunNamed("vendor:publish"); got != legacyPHPDisabled {
		t.Fatalf("vendor:publish=%q", got)
	}
}

func TestPHPScaffoldIgnoredEvenWhenEnabled(t *testing.T) {
	t.Setenv("LIKEADMIN_ENABLE_PHP_SCAFFOLD", "1")
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	t.Cleanup(func() { config.C.App.PublicDir = old })
	if got := RunNamed("make:controller", "tenantapi@Demo"); got != legacyPHPDisabled {
		t.Fatalf("make:controller=%q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "app", "tenantapi", "controller", "DemoController.php")); err == nil {
		t.Fatal("PHP controller must not be written")
	}
	if got := RunNamed("vendor:publish"); got != legacyPHPDisabled {
		t.Fatalf("vendor:publish=%q", got)
	}
	if got := RunNamed("build", "demo"); got != legacyPHPDisabled {
		t.Fatalf("build=%q", got)
	}
}

func TestThinkHookExecAndRejectPHP(t *testing.T) {
	dir := t.TempDir()
	echo, err := exec.LookPath("echo")
	if err != nil {
		t.Skip(err)
	}
	yml := filepath.Join(dir, "think-commands.yaml")
	body := "commands:\n  hook_echo:\n    description: Echo hook\n    program: " + echo + "\n    args: [\"hook-ok\"]\n  hook_php:\n    program: /usr/bin/php\n    args: [\"think\", \"clear\"]\n"
	if err := os.WriteFile(yml, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LIKEADMIN_THINK_COMMANDS", yml)
	resetThinkHooksForTest()
	t.Cleanup(resetThinkHooksForTest)

	if got := runCommand(model.Crontab{Command: "hook_echo"}); got != "" {
		t.Fatalf("echo hook %q", got)
	}
	if got := runCommand(model.Crontab{Command: "hook_php"}); got != "拒绝执行 PHP 命令: hook_php" {
		t.Fatalf("php hook %q", got)
	}
	raw := formatCommandList(true, "")
	if !strings.Contains(raw, "hook_echo") {
		t.Fatalf("list missing hook_echo: %s", raw)
	}
	if strings.Contains(raw, "hook_php") {
		t.Fatalf("list should omit php hook: %s", raw)
	}
	if got := RunNamed("help", "hook_echo"); got != "" {
		t.Fatalf("help hook %q", got)
	}
}
