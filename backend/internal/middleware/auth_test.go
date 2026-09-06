package middleware

import "testing"

func TestFormatURIPerms(t *testing.T) {
	if got := formatURI("auth.admin/lists"); got != "auth.admin/lists" {
		t.Fatalf("%q", got)
	}
	if got := formatURI("user.user/adjustMoney"); got != "user.user/adjustmoney" {
		t.Fatalf("%q", got)
	}
	if got := formatURI("user.user/adjust_money"); got != "user.user/adjustmoney" {
		t.Fatalf("%q", got)
	}
	if !containsURI([]string{"user.user/adjustmoney"}, "user.user/adjustMoney") {
		t.Fatal("camel action should match")
	}
}

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
