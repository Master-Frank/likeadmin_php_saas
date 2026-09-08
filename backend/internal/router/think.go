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
	root := ""
	if pub := config.C.App.PublicDir; pub != "" {
		root = filepath.Dir(pub)
	}
	if root == "" {
		root = "server"
	}
	dir := filepath.Join(root, "runtime")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err.Error()
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		return err.Error()
	}
	return ""
}
