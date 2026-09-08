package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const restartCommandEnv = "LIKEADMIN_UPGRADE_RESTART_COMMAND"

type stagedGoUpgrade struct {
	api, crontab string
}

func stageGoUpgrade(tempDir, liveBackend string) (*stagedGoUpgrade, error) {
	patch := filepath.Join(tempDir, "project", "backend")
	if st, err := os.Stat(patch); err != nil || !st.IsDir() {
		return nil, nil
	}
	stage, err := os.MkdirTemp("", "likeadmin-go-upgrade-*")
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(stage)
		}
	}()
	src := filepath.Join(stage, "backend")
	if err := copyGoTree(liveBackend, src); err != nil {
		return nil, fmt.Errorf("准备 Go 升级源码失败: %w", err)
	}
	if err := upgradeFile(patch, src); err != nil {
		return nil, err
	}
	bin := filepath.Join(stage, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	for _, target := range []string{"api", "crontab"} {
		cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-o", filepath.Join(bin, target), "./cmd/"+target)
		cmd.Dir = src
		cmd.Env = os.Environ()
		if out, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("构建 Go %s 失败: %v: %s", target, err, strings.TrimSpace(string(out)))
		}
	}
	cleanup = false
	return &stagedGoUpgrade{api: filepath.Join(bin, "api"), crontab: filepath.Join(bin, "crontab")}, nil
}

func (s *stagedGoUpgrade) cleanup() {
	if s != nil {
		_ = os.RemoveAll(filepath.Dir(filepath.Dir(s.api)))
	}
}

func (s *stagedGoUpgrade) install(liveBackend string) error {
	if s == nil {
		return nil
	}
	bin := filepath.Join(liveBackend, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		return err
	}
	for _, item := range []struct{ src, name string }{{s.api, "api"}, {s.crontab, "crontab"}} {
		dst := filepath.Join(bin, item.name)
		tmp := dst + ".upgrade"
		if err := copyFile(item.src, tmp, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			_ = os.Remove(tmp)
			return err
		}
	}
	return nil
}

func requireRestartCommand() error {
	if strings.TrimSpace(os.Getenv(restartCommandEnv)) == "" {
		return fmt.Errorf("升级包含 Go 后端，但未配置 %s", restartCommandEnv)
	}
	return nil
}

func runRestartCommand() error {
	raw := strings.TrimSpace(os.Getenv(restartCommandEnv))
	var argv []string
	if err := json.Unmarshal([]byte(raw), &argv); err != nil || len(argv) == 0 || !filepath.IsAbs(argv[0]) {
		return fmt.Errorf("%s 必须是绝对程序路径开头的 JSON 数组", restartCommandEnv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("重启 Go 服务失败: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func copyGoTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		top := strings.Split(filepath.ToSlash(rel), "/")[0]
		if d.IsDir() && (top == "bin" || top == "runtime" || top == ".git") {
			return filepath.SkipDir
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, target, info.Mode().Perm())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
