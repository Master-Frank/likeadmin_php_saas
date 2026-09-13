package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
)

func TestRestoreIndexFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("lock-page"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index_lock.html"), []byte("spa"), 0644); err != nil {
		t.Fatal(err)
	}
	restoreIndexFile(dir)
	got, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "spa" {
		t.Fatalf("got %s", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "index_lock.html")); err == nil {
		t.Fatal("lock file should be gone")
	}
}

func TestProbeDir(t *testing.T) {
	item := probeDir("runtime", t.TempDir())
	if item.Status != "ok" {
		t.Fatalf("%+v", item)
	}
	item = probeDir("missing", "")
	if item.Status != "fail" {
		t.Fatal(item)
	}
}

func TestProbeWritableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	item := probeWritableFile("config.yaml", path)
	if item.Status != "ok" {
		t.Fatalf("%+v", item)
	}
	missing := probeWritableFile("config.yaml", filepath.Join(dir, "missing.yaml"))
	if missing.Status != "fail" {
		t.Fatalf("missing yaml must fail: %+v", missing)
	}
	empty := probeWritableFile("config.yaml", "")
	if empty.Status != "fail" || empty.Value != "未配置" {
		t.Fatalf("%+v", empty)
	}
}

func TestFormatDiskSpace(t *testing.T) {
	if got := formatDiskSpace(512 * 1024 * 1024); got != "512.00M" {
		t.Fatalf("512M %s", got)
	}
	if got := formatDiskSpace(1536 * 1024 * 1024); got != "1.50G" {
		t.Fatalf("1.5G %s", got)
	}
}

func TestProbeDiskSpace(t *testing.T) {
	item := probeDiskSpace()
	if item.Name != "磁盘空间" || item.Status != "ok" || item.Value == "" {
		t.Fatalf("%+v", item)
	}
	if !strings.HasSuffix(item.Value, "G") && !strings.HasSuffix(item.Value, "M") {
		t.Fatalf("unit %s", item.Value)
	}
}

func TestCollectEnvServerInfo(t *testing.T) {
	items := CollectEnv()
	names := map[string]string{}
	for _, it := range items {
		names[it.Name] = it.Value
	}
	for _, name := range []string{"服务器操作系统", "web服务器环境", "程序安装目录", "上传限制", "public/uploads", "public/mobile", "config.yaml"} {
		if _, ok := names[name]; !ok {
			t.Fatalf("missing %s in %+v", name, names)
		}
	}
	if names["服务器操作系统"] == "" || names["程序安装目录"] == "" {
		t.Fatalf("empty server info %+v", names)
	}
}

func TestProbeMySQLNilIsSoft(t *testing.T) {
	if bootstrap.DB != nil {
		t.Skip("bootstrap.DB already connected")
	}
	item := probeMySQL()
	if item.Status != "ok" || item.Value != "安装时填写" {
		t.Fatalf("%+v", item)
	}
}

func TestEnvBlockingWritableTree(t *testing.T) {
	root := t.TempDir()
	pub := filepath.Join(root, "public")
	for _, d := range []string{"uploads", "platform", "admin", "mobile"} {
		if err := os.MkdirAll(filepath.Join(pub, d), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "runtime"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(root, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(yamlPath, []byte("app:\n  debug: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldPub, oldLock, oldPath := config.C.App.PublicDir, config.C.App.InstallLock, config.Path
	t.Cleanup(func() {
		config.C.App.PublicDir = oldPub
		config.C.App.InstallLock = oldLock
		config.Path = oldPath
	})
	config.C.App.PublicDir = pub
	config.C.App.InstallLock = filepath.Join(cfgDir, "install.lock")
	config.Path = yamlPath
	if msg := EnvBlocking(); msg != "" {
		t.Fatalf("writable tree: %s", msg)
	}
}

func TestPublicSub(t *testing.T) {
	if got := publicSub("uploads"); !strings.HasSuffix(got, "uploads") {
		t.Fatalf("got %s", got)
	}
}
