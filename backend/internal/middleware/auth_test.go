package middleware

import "testing"

func TestRejectWrongTenant(t *testing.T) {
	if rejectWrongTenant(false, 1, 2) {
		t.Fatal("optional login should allow a stale cross-tenant token")
	}
	if !rejectWrongTenant(true, 1, 2) {
		t.Fatal("required login should reject a cross-tenant token")
	}
	if rejectWrongTenant(true, 1, 1) {
		t.Fatal("same tenant should pass")
	}
	if rejectWrongTenant(true, 1, 0) {
		t.Fatal("unset host tenant should pass")
	}
}
