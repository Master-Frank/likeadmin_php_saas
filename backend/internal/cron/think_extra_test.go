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

func TestMakeControllerAndModel(t *testing.T) {
	t.Setenv("LIKEADMIN_ENABLE_PHP_SCAFFOLD", "1")
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	t.Cleanup(func() { config.C.App.PublicDir = old })

	if got := RunNamed("make:controller", "tenantapi@Demo"); got != "" {
		t.Fatal(got)
	}
	path := filepath.Join(dir, "app", "tenantapi", "controller", "DemoController.php")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "namespace app\\tenantapi\\controller;") || !strings.Contains(text, "class DemoController") {
		t.Fatalf("controller %s", text)
	}
	if !strings.Contains(text, "function index()") || !strings.Contains(text, "function create()") {
		t.Fatal("resource stub")
	}
	if got := RunNamed("make:controller", "tenantapi@Demo"); !strings.Contains(got, "already exists") {
		t.Fatalf("dup %q", got)
	}
	if got := RunNamed("make:controller", "--api", "Ping"); got != "" {
		t.Fatal(got)
	}
	api, err := os.ReadFile(filepath.Join(dir, "app", "controller", "PingController.php"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(api), "function create()") {
		t.Fatal("api stub should omit create")
	}
	if got := RunNamed("make:model", "tenantapi@Demo"); got != "" {
		t.Fatal(got)
	}
	modelBody, err := os.ReadFile(filepath.Join(dir, "app", "tenantapi", "model", "Demo.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modelBody), "class Demo extends Model") {
		t.Fatalf("model %s", modelBody)
	}
	if got := RunNamed("make:command", "Foo", "foo:bar"); got != "" {
		t.Fatal(got)
	}
	cmd, err := os.ReadFile(filepath.Join(dir, "app", "command", "Foo.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(cmd), "setName('foo:bar')") {
		t.Fatalf("command %s", cmd)
	}
	if got := RunNamed("make:controller"); got != `Not enough arguments (missing: "name").` {
		t.Fatalf("missing name %q", got)
	}
}

func TestVendorPublishAndServiceDiscover(t *testing.T) {
	t.Setenv("LIKEADMIN_ENABLE_PHP_SCAFFOLD", "1")
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	t.Cleanup(func() { config.C.App.PublicDir = old })

	pkgDir := filepath.Join(dir, "vendor", "acme", "demo")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "vendor", "composer"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "src-config.php"), []byte("<?php return [];"), 0644); err != nil {
		t.Fatal(err)
	}
	installed := `{
  "packages": [{
    "name": "acme/demo",
    "extra": {
      "think": {
        "services": ["think\\app\\Service", "acme\\DemoService"],
        "config": {"demo": "src-config.php"}
      }
    }
  }]
}`
	if err := os.WriteFile(filepath.Join(dir, "vendor", "composer", "installed.json"), []byte(installed), 0644); err != nil {
		t.Fatal(err)
	}
	if got := RunNamed("vendor:publish"); got != "" {
		t.Fatal(got)
	}
	copied, err := os.ReadFile(filepath.Join(dir, "config", "demo.php"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != "<?php return [];" {
		t.Fatalf("copied %s", copied)
	}
	if got := RunNamed("service:discover"); got != "" {
		t.Fatal(got)
	}
	services, err := os.ReadFile(filepath.Join(dir, "vendor", "services.php"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(services)
	if !strings.Contains(text, "<?php") || !strings.Contains(text, `think\\app\\Service`) || !strings.Contains(text, `acme\\DemoService`) {
		t.Fatalf("services %s", text)
	}
}

func TestBuildAppDirs(t *testing.T) {
	t.Setenv("LIKEADMIN_ENABLE_PHP_SCAFFOLD", "1")
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	t.Cleanup(func() { config.C.App.PublicDir = old })
	if got := RunNamed("build", "demo"); got != "" {
		t.Fatal(got)
	}
	for _, p := range []string{
		filepath.Join(dir, "app", "demo", "controller"),
		filepath.Join(dir, "app", "demo", "model"),
		filepath.Join(dir, "app", "demo", "view"),
	} {
		if st, err := os.Stat(p); err != nil || !st.IsDir() {
			t.Fatalf("missing dir %s", p)
		}
	}
	hello, err := os.ReadFile(filepath.Join(dir, "app", "demo", "controller", "IndexController.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(hello), "namespace app\\demo\\controller;") {
		t.Fatalf("hello %s", hello)
	}
	common, err := os.ReadFile(filepath.Join(dir, "app", "demo", "common.php"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(common), "系统自动生成的公共文件") {
		t.Fatalf("common %s", common)
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
