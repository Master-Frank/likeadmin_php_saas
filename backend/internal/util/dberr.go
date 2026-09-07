package util

import "strings"

// IsDuplicateKey reports MySQL / SQLite unique-constraint errors (SQLSTATE 23000 / 1062).
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "1062") ||
		strings.Contains(s, "Duplicate") ||
		strings.Contains(s, "UNIQUE constraint failed") ||
		strings.Contains(s, "duplicate key")
}
