package platformapi

import "testing"

func TestValidTenantSN(t *testing.T) {
	if !validTenantSN("pair2") || !validTenantSN("abc123") {
		t.Fatal("valid sn rejected")
	}
	if !validTenantSN("UPPER") {
		t.Fatal("uppercase sn rejected")
	}
	if validTenantSN("") || validTenantSN("bad-sn") || validTenantSN("a_b") {
		t.Fatal("invalid sn accepted")
	}
}
