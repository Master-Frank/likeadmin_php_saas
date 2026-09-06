package generator

import (
	"strings"
	"unicode"

	"likeadmin/backend/internal/config"
)

// NoPrefix strips the configured table prefix from the start of a table name.
func NoPrefix(table string) string {
	prefix := config.Prefix()
	if prefix != "" && strings.HasPrefix(table, prefix) {
		return strings.TrimPrefix(table, prefix)
	}
	return table
}

// Studly matches ThinkPHP Str::studly: foo_bar / foo-bar -> FooBar.
func Studly(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	parts := strings.Fields(s)
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

// Camel matches ThinkPHP Str::camel: foo_bar -> fooBar.
func Camel(s string) string {
	studly := Studly(s)
	if studly == "" {
		return ""
	}
	r := []rune(studly)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// Lower matches ThinkPHP Str::lower.
func Lower(s string) string {
	return strings.ToLower(s)
}

func phpTruthy(v int) bool { return v != 0 }

func setBlankSpace(content, blank string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = blank + line
	}
	return strings.Join(lines, "\n")
}

func phpSubstr(s string, start, length int) string {
	if s == "" {
		return ""
	}
	if start < 0 {
		start = len(s) + start
		if start < 0 {
			start = 0
		}
	}
	if start > len(s) {
		return ""
	}
	if length < 0 {
		end := len(s) + length
		if end <= start {
			return ""
		}
		return s[start:end]
	}
	end := start + length
	if end > len(s) {
		end = len(s)
	}
	return s[start:end]
}
