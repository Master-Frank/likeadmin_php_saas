package util

import (
	"strings"
	"unicode"
)

// LowerURI mirrors PHP lower_uri(): strtolower(Str::camel($item)).
func LowerURI(s string) string {
	return strings.ToLower(camelPHP(s))
}

func camelPHP(s string) string {
	s = studlyPHP(s)
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func studlyPHP(s string) string {
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")
	return strings.ReplaceAll(ucwordsPHP(s), " ", "")
}

func ucwordsPHP(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range s {
		if r == ' ' {
			prevSpace = true
			b.WriteRune(r)
			continue
		}
		if prevSpace {
			b.WriteRune(unicode.ToUpper(r))
			prevSpace = false
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
