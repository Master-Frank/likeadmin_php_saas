package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

type envItem struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Value  string `json:"value"`
}

func Env(c *gin.Context) {
	items := CollectEnv()
	ok := true
	for _, it := range items {
		if it.Status != "ok" {
			ok = false
			break
		}
	}
	lock := config.C.App.InstallLock
	installed := lock != ""
	if lock != "" {
		if _, err := os.Stat(lock); err != nil {
			installed = false
		}
	}
	response.Data(c, gin.H{"ok": ok, "installed": installed, "items": items})
}

func CollectEnv() []envItem {
	out := []envItem{
		{Name: "Go", Status: "ok", Value: runtime.Version()},
		{Name: "服务器操作系统", Status: "ok", Value: runtime.GOOS + "/" + runtime.GOARCH},
		{Name: "web服务器环境", Status: "ok", Value: "Go net/http"},
		{Name: "程序安装目录", Status: "ok", Value: installRoot()},
	}
	out = append(out, probeMySQL())
	out = append(out, probeRedis())
	out = append(out, probeDir("runtime", runtimeDir()))
	out = append(out, probeDir("public", config.C.App.PublicDir))
	out = append(out, probeDir("public/uploads", publicSub("uploads")))
	out = append(out, probeDir("public/platform", publicSub("platform")))
	out = append(out, probeDir("public/admin", publicSub("admin")))
	out = append(out, probeDir("public/mobile", publicSub("mobile")))
	out = append(out, probeDir("config", configDir()))
	out = append(out, probeWritableFile(".env", envFilePath()))
	out = append(out, probeDiskSpace())
	out = append(out, probeUploadLimit())
	return out
}

func installRoot() string {
	dir := config.C.App.PublicDir
	if dir == "" {
		dir = "."
	} else {
		dir = filepath.Dir(dir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

func probeUploadLimit() envItem {
	item := envItem{Name: "上传限制", Status: "ok", Value: "由反向代理与应用配置决定"}
	if v := strings.TrimSpace(os.Getenv("LIKEADMIN_UPLOAD_MAX")); v != "" {
		item.Value = v
	}
	return item
}

func probeMySQL() envItem {
	item := envItem{Name: "MySQL", Status: "fail", Value: "未连接"}
	if bootstrap.DB == nil {
		return item
	}
	sqlDB, err := bootstrap.DB.DB()
	if err != nil {
		item.Value = err.Error()
		return item
	}
	if err := sqlDB.Ping(); err != nil {
		item.Value = err.Error()
		return item
	}
	item.Status = "ok"
	item.Value = config.C.Database.Hostname
	return item
}

func probeRedis() envItem {
	item := envItem{Name: "Redis", Status: "fail", Value: "未连接"}
	if bootstrap.RDB == nil {
		item.Status = "ok"
		item.Value = "未配置（使用内存缓存）"
		return item
	}
	if err := bootstrap.RDB.Ping(context.Background()).Err(); err != nil {
		item.Value = err.Error()
		return item
	}
	item.Status = "ok"
	item.Value = config.C.Redis.Host
	return item
}

func probeDir(name, dir string) envItem {
	item := envItem{Name: name, Status: "fail", Value: dir}
	if dir == "" {
		item.Value = "未配置"
		return item
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		item.Value = err.Error()
		return item
	}
	probe := filepath.Join(dir, ".write_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		item.Value = err.Error()
		return item
	}
	_ = os.Remove(probe)
	item.Status = "ok"
	return item
}

func probeWritableFile(name, path string) envItem {
	item := envItem{Name: name, Status: "fail", Value: path}
	if path == "" {
		item.Value = "未配置"
		return item
	}
	info, err := os.Stat(path)
	if err != nil {
		item.Value = "文件不存在"
		return item
	}
	if info.IsDir() {
		item.Value = "不是文件"
		return item
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		item.Value = err.Error()
		return item
	}
	_ = f.Close()
	item.Status = "ok"
	return item
}

// probeDiskSpace mirrors PHP installModel::freeDiskSpace on the project root.
func probeDiskSpace() envItem {
	item := envItem{Name: "磁盘空间", Status: "ok"}
	dir := config.C.App.PublicDir
	if dir == "" {
		dir = "."
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	var st syscall.Statfs_t
	if err := syscall.Statfs(dir, &st); err != nil {
		item.Status = "fail"
		item.Value = err.Error()
		return item
	}
	item.Value = formatDiskSpace(float64(st.Bavail) * float64(st.Bsize))
	return item
}

func formatDiskSpace(bytes float64) string {
	mb := bytes / 1024 / 1024
	if mb > 1024 {
		return fmt.Sprintf("%.2fG", mb/1024)
	}
	return fmt.Sprintf("%.2fM", mb)
}

func publicSub(name string) string {
	if config.C.App.PublicDir != "" {
		return filepath.Join(config.C.App.PublicDir, name)
	}
	return filepath.Join("public", name)
}

func envFilePath() string {
	if config.C.App.InstallLock != "" {
		return filepath.Join(filepath.Dir(config.C.App.InstallLock), "..", ".env")
	}
	if config.C.App.PublicDir != "" {
		return filepath.Join(config.C.App.PublicDir, "..", ".env")
	}
	return ".env"
}

func runtimeDir() string {
	if config.C.App.PublicDir != "" {
		return filepath.Join(config.C.App.PublicDir, "..", "runtime")
	}
	return "runtime"
}

func configDir() string {
	if config.C.App.InstallLock != "" {
		return filepath.Dir(config.C.App.InstallLock)
	}
	if config.C.App.PublicDir != "" {
		return filepath.Join(config.C.App.PublicDir, "..", "config")
	}
	return "config"
}

func restoreIndexLock() {
	pub := config.C.App.PublicDir
	if pub == "" {
		return
	}
	for _, dir := range []string{"admin", "mobile"} {
		restoreIndexFile(filepath.Join(pub, dir))
	}
}

func restoreIndexFile(dir string) {
	lock := filepath.Join(dir, "index_lock.html")
	if _, err := os.Stat(lock); err != nil {
		return
	}
	idx := filepath.Join(dir, "index.html")
	_ = os.Remove(idx)
	_ = os.Rename(lock, idx)
}
