package cron

import (
	"fmt"
	"sort"
	"strings"
)

// CLI extras that work from cmd/think but are not Register()'d jobs
// (crontab would recurse; upgrade-local lives in cmd/think).
var cliExtraNames = []string{"crontab", "upgrade-local"}

var commandDescs = map[string]string{
	"cache":                "Flush application cache",
	"cancel_unpaid_orders": "Cancel unpaid recharge orders",
	"clear":                "Clear runtime file",
	"crontab":              "Run scheduled tasks once",
	"help":                 "Displays help for a command",
	"list":                 "Lists commands",
	"optimize:route":       "Build route cache",
	"optimize:schema":      "Build schema cache",
	"query_refund":         "Query refund status",
	"route:list":           "List application routes",
	"session":              "Expire stale login sessions",
	"upgrade-local":        "Apply a local upgrade zip",
	"version":              "Show think framework version",
}

func allCLINames() []string {
	seen := map[string]bool{}
	out := CommandNames()
	for _, n := range out {
		seen[n] = true
	}
	for _, n := range cliExtraNames {
		if !seen[n] {
			out = append(out, n)
			seen[n] = true
		}
	}
	sort.Strings(out)
	return out
}

func commandDesc(name string) string {
	if d := commandDescs[name]; d != "" {
		return d
	}
	return ""
}

func filterCLINames(names []string, ns string) []string {
	if ns == "" {
		return names
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n == ns || strings.HasPrefix(n, ns+":") {
			out = append(out, n)
		}
	}
	return out
}

func formatCommandList(raw bool, ns string) string {
	names := filterCLINames(allCLINames(), ns)
	var b strings.Builder
	if raw {
		for _, n := range names {
			b.WriteString(n)
			b.WriteByte('\n')
		}
		return b.String()
	}
	b.WriteString("Available commands:\n")
	for _, n := range names {
		if d := commandDesc(n); d != "" {
			fmt.Fprintf(&b, "  %-24s %s\n", n, d)
		} else {
			fmt.Fprintf(&b, "  %s\n", n)
		}
	}
	return b.String()
}

func runList(args []string) string {
	raw := false
	ns := ""
	for _, a := range args {
		switch {
		case a == "--raw":
			raw = true
		case !strings.HasPrefix(a, "-"):
			if ns == "" {
				ns = a
			}
		}
	}
	fmt.Print(formatCommandList(raw, ns))
	return ""
}

func runHelp(args []string) string {
	name := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			name = a
			break
		}
	}
	if name == "" {
		return runList(nil)
	}
	key := normalizeCommand(name)
	known := commandDesc(key) != ""
	if !known {
		commandMu.RLock()
		_, known = commands[key]
		commandMu.RUnlock()
	}
	if !known {
		for _, n := range cliExtraNames {
			if n == key || n == name {
				known = true
				key = n
				break
			}
		}
	}
	if !known {
		return fmt.Sprintf("未定义的命令: %s", name)
	}
	fmt.Println(key)
	if d := commandDesc(key); d != "" {
		fmt.Println(d)
	}
	return ""
}
