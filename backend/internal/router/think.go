package router

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/cron"
)

func init() {
	cron.Register("route:list", runRouteList)
	cron.Register("optimize:route", runOptimizeRoute)
}

func runRouteList([]string) string {
	return writeThinkRouteFile("route_list.php", thinkRouteListing())
}

func runOptimizeRoute([]string) string {
	return writeThinkRouteFile("route.php", "<?php\nreturn "+thinkRoutePHPArray()+";\n")
}

func thinkRouteListing() string {
	var b strings.Builder
	b.WriteString("Route List\n")
	for _, app := range []struct {
		name string
		keys []string
	}{
		{"platformapi", sortedRouteKeys(platformRoutes())},
		{"tenantapi", sortedRouteKeys(tenantRoutes())},
		{"api", sortedRouteKeys(apiRoutes())},
	} {
		for _, key := range app.keys {
			b.WriteString(app.name)
			b.WriteByte('\t')
			b.WriteString(key)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func thinkRoutePHPArray() string {
	var b strings.Builder
	b.WriteString("[\n")
	for _, app := range []string{"platformapi", "tenantapi", "api"} {
		var keys []string
		switch app {
		case "platformapi":
			keys = sortedRouteKeys(platformRoutes())
		case "tenantapi":
			keys = sortedRouteKeys(tenantRoutes())
		default:
			keys = sortedRouteKeys(apiRoutes())
		}
		for _, key := range keys {
			b.WriteString("  '")
			b.WriteString(app)
			b.WriteString("/")
			b.WriteString(key)
			b.WriteString("',\n")
		}
	}
	b.WriteString("]")
	return b.String()
}

func sortedRouteKeys(routes map[string]Handler) []string {
	out := make([]string, 0, len(routes))
	for k := range routes {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func writeThinkRouteFile(name, content string) string {
	var lastErr string
	wrote := false
	for _, dir := range thinkRuntimeDirs() {
		if err := os.MkdirAll(dir, 0755); err != nil {
			lastErr = err.Error()
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			lastErr = err.Error()
			continue
		}
		wrote = true
	}
	if !wrote {
		if lastErr == "" {
			return "runtime directory unavailable"
		}
		return lastErr
	}
	return ""
}

func thinkRuntimeDirs() []string {
	seen := map[string]bool{}
	var out []string
	add := func(dir string) {
		if dir == "" || seen[dir] {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	}
	if pub := config.C.App.PublicDir; pub != "" {
		root := filepath.Dir(pub)
		if st, err := os.Stat(root); err == nil && st.IsDir() {
			add(filepath.Join(root, "runtime"))
		}
	}
	wd, _ := os.Getwd()
	for d := wd; d != "" && d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "internal", "cron")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
				add(filepath.Join(d, "runtime"))
				break
			}
		}
		if d == filepath.Dir(d) {
			break
		}
	}
	return out
}
