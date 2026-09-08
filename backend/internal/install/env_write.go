package install

import (
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/config"
)

type envPair struct {
	Key string
	Val string
}

// WriteEnv merges install values into server/.example.env the way PHP YxEnv::putEnv does.
func WriteEnv(path string, host, dbName, user, pass string, port int, prefix, httpHost, uniqueID string) error {
	if prefix == "" {
		prefix = "la_"
	}
	if port == 0 {
		port = 3306
	}
	if uniqueID == "" {
		uniqueID = "likeadmin"
	}
	if err := makeEnv(path); err != nil {
		return err
	}
	pairs := loadExampleEnv(path)
	if len(pairs) == 0 {
		pairs = defaultEnvPairs()
	}
	pairs = setEnv(pairs, "HTTP_HOST", httpHost)
	pairs = setEnv(pairs, "DATABASE.HOSTNAME", host)
	pairs = setEnv(pairs, "DATABASE.DATABASE", dbName)
	pairs = setEnv(pairs, "DATABASE.USERNAME", user)
	pairs = setEnv(pairs, "DATABASE.PASSWORD", pass)
	pairs = setEnv(pairs, "DATABASE.HOSTPORT", itoa(port))
	pairs = setEnv(pairs, "DATABASE.PREFIX", prefix)
	pairs = setEnv(pairs, "PROJECT.UNIQUE_IDENTIFICATION", uniqueID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(formatPHPEnv(pairs)), 0o644)
}

// makeEnv mirrors PHP YxEnv::makeEnv: touch .env if missing.
func makeEnv(path string) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}

func loadExampleEnv(envPath string) []envPair {
	for _, p := range exampleEnvCandidates(envPath) {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if pairs := parseINI(string(b)); len(pairs) > 0 {
			return pairs
		}
	}
	return nil
}

func exampleEnvCandidates(envPath string) []string {
	out := []string{}
	if v := strings.TrimSpace(os.Getenv("LIKEADMIN_EXAMPLE_ENV")); v != "" {
		out = append(out, v)
	}
	if envPath != "" {
		out = append(out, filepath.Join(filepath.Dir(envPath), ".example.env"))
	}
	if config.C.App.PublicDir != "" {
		out = append(out, filepath.Join(filepath.Dir(config.C.App.PublicDir), ".example.env"))
	}
	out = append(out, "/workspace/server/.example.env")
	return out
}

func defaultEnvPairs() []envPair {
	return []envPair{
		{Key: "APP_DEBUG", Val: "1"},
		{Key: "APP.DEFAULT_TIMEZONE", Val: "Asia/Shanghai"},
		{Key: "DATABASE.TYPE", Val: "mysql"},
		{Key: "DATABASE.HOSTNAME", Val: "127.0.0.1"},
		{Key: "DATABASE.DATABASE", Val: "test"},
		{Key: "DATABASE.USERNAME", Val: "username"},
		{Key: "DATABASE.PASSWORD", Val: "password"},
		{Key: "DATABASE.HOSTPORT", Val: "3306"},
		{Key: "DATABASE.CHARSET", Val: "utf8mb4"},
		{Key: "DATABASE.DEBUG", Val: "1"},
		{Key: "DATABASE.PREFIX", Val: "la_"},
		{Key: "LANG.default_lang", Val: "zh-cn"},
		{Key: "PROJECT.UNIQUE_IDENTIFICATION", Val: "likeadmin"},
		{Key: "PROJECT.DEFAULT_PASSWORD", Val: "123456"},
		{Key: "PROJECT.DEMO_ENV", Val: ""},
	}
}

func setEnv(pairs []envPair, key, val string) []envPair {
	for i := range pairs {
		if pairs[i].Key == key {
			pairs[i].Val = val
			return pairs
		}
	}
	return append(pairs, envPair{Key: key, Val: val})
}

func parseINI(raw string) []envPair {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	section := ""
	out := []envPair{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		v = strings.Trim(v, `"'`)
		v = phpINIBool(v)
		if k == "" {
			continue
		}
		key := k
		if section != "" {
			key = section + "." + k
		}
		out = append(out, envPair{Key: key, Val: v})
	}
	return out
}

func phpINIBool(v string) string {
	switch strings.ToLower(v) {
	case "true", "on", "yes":
		return "1"
	case "false", "off", "no", "none":
		return ""
	default:
		return v
	}
}

func formatPHPEnv(pairs []envPair) string {
	var b strings.Builder
	lastPrefix := ""
	for _, p := range pairs {
		prefix, key, dotted := strings.Cut(p.Key, ".")
		if dotted {
			if prefix != lastPrefix {
				if lastPrefix != "" {
					b.WriteByte('\n')
				}
				b.WriteByte('[')
				b.WriteString(prefix)
				b.WriteString("]\n")
				lastPrefix = prefix
			}
			b.WriteString(key)
			b.WriteString(" = \"")
			b.WriteString(p.Val)
			b.WriteString("\"\n")
			continue
		}
		b.WriteString(p.Key)
		b.WriteString(" = \"")
		b.WriteString(p.Val)
		b.WriteString("\"\n")
	}
	return b.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
