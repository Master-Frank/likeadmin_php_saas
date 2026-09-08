package util

import (
	"errors"
	"testing"
)

func TestIsDuplicateKey(t *testing.T) {
	if IsDuplicateKey(nil) {
		t.Fatal("nil")
	}
	if !IsDuplicateKey(errors.New("Error 1062 (23000): Duplicate entry 'u1' for key 'account'")) {
		t.Fatal("mysql 1062")
	}
	if !IsDuplicateKey(errors.New("UNIQUE constraint failed: la_user.account")) {
		t.Fatal("sqlite unique")
	}
	if IsDuplicateKey(errors.New("connection refused")) {
		t.Fatal("other error")
	}
}
