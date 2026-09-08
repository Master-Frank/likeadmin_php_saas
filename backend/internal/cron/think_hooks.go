package cron

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type thinkHook struct {
	Description string   `yaml:"description"`
	Program     string   `yaml:"program"`
	Args        []string `yaml:"args"`
}

type thinkCommandsFile struct {
	Commands map[string]thinkHook `yaml:"commands"`
}

var (
	hookMu      sync.Mutex
	thinkHooks  map[string]thinkHook
	hooksLoaded bool
)

func thinkCommandsPath() string {
	if p := strings.TrimSpace(os.Getenv("LIKEADMIN_THINK_COMMANDS")); p != "" {
		return p
	}
	if cfg := strings.TrimSpace(os.Getenv("LIKEADMIN_CONFIG")); cfg != "" {
		return filepath.Join(filepath.Dir(cfg), "think-commands.yaml")
	}
	root := runtimeRoot()
	if root != "" {
		cand := filepath.Join(filepath.Dir(root), "backend", "configs", "think-commands.yaml")
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return filepath.Join("backend", "configs", "think-commands.yaml")
}

func resetThinkHooksForTest() {
	hookMu.Lock()
	defer hookMu.Unlock()
	thinkHooks = nil
	hooksLoaded = false
}

func ensureThinkHooks() {
	hookMu.Lock()
	defer hookMu.Unlock()
	if hooksLoaded {
		return
	}
	hooksLoaded = true
	thinkHooks = map[string]thinkHook{}
	path := thinkCommandsPath()
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var file thinkCommandsFile
	if err := yaml.Unmarshal(b, &file); err != nil {
		return
	}
	for name, h := range file.Commands {
		key := strings.TrimSpace(name)
		if key == "" {
			continue
		}
		thinkHooks[key] = h
	}
}

func lookupThinkHook(name string) (thinkHook, bool) {
	ensureThinkHooks()
	hookMu.Lock()
	defer hookMu.Unlock()
	h, ok := thinkHooks[name]
	return h, ok
}

func hookDescription(name string) string {
	h, ok := lookupThinkHook(name)
	if !ok {
		return ""
	}
	return strings.TrimSpace(h.Description)
}

func hookCLINames() []string {
	ensureThinkHooks()
	hookMu.Lock()
	defer hookMu.Unlock()
	out := make([]string, 0, len(thinkHooks))
	for name, h := range thinkHooks {
		if looksLikePHP(h.Program, h.Args) {
			continue
		}
		if strings.TrimSpace(h.Program) == "" || !filepath.IsAbs(h.Program) {
			continue
		}
		out = append(out, name)
	}
	return out
}

func runThinkHook(name string, args []string) (string, bool) {
	h, ok := lookupThinkHook(name)
	if !ok {
		return "", false
	}
	if looksLikePHP(h.Program, h.Args) {
		return fmt.Sprintf("拒绝执行 PHP 命令: %s", name), true
	}
	prog := strings.TrimSpace(h.Program)
	if prog == "" || !filepath.IsAbs(prog) {
		return fmt.Sprintf("未定义的定时任务命令: %s", name), true
	}
	return execThinkHook(h, args), true
}

func looksLikePHP(program string, args []string) bool {
	tokens := append([]string{program}, args...)
	for _, t := range tokens {
		base := strings.ToLower(filepath.Base(strings.TrimSpace(t)))
		switch base {
		case "php", "php.exe", "php-fpm", "php-cgi", "php-cli":
			return true
		}
	}
	return false
}

func execThinkHook(h thinkHook, extra []string) string {
	args := append(append([]string{}, h.Args...), extra...)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, h.Program, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return msg
	}
	if len(out) > 0 {
		fmt.Print(string(out))
	}
	return ""
}
