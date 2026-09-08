package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/cron"
)

func TestThinkRouteListAndOptimizeRoute(t *testing.T) {
	if config.C.App.PublicDir == "" {
		cfg := os.Getenv("LIKEADMIN_CONFIG")
		if cfg == "" {
			cfg = "/workspace/backend/configs/config.yaml"
		}
		_ = bootstrap.Init(cfg)
	}
	if got := cron.RunNamed("route:list"); got != "" {
		t.Fatal(got)
	}
	if got := cron.RunNamed("optimize:route"); got != "" {
		t.Fatal(got)
	}
	root := filepath.Dir(config.C.App.PublicDir)
	listPath := filepath.Join(root, "runtime", "route_list.php")
	optPath := filepath.Join(root, "runtime", "route.php")
	t.Cleanup(func() {
		_ = os.Remove(listPath)
		_ = os.Remove(optPath)
	})
	list, err := os.ReadFile(listPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(list), "platformapi\tlogin/account") || !strings.Contains(string(list), "api\tlogin/register") {
		t.Fatalf("route_list %s", list)
	}
	opt, err := os.ReadFile(optPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(opt), "<?php") || !strings.Contains(string(opt), "tenantapi/login/account") {
		t.Fatalf("route.php %s", opt)
	}
}
