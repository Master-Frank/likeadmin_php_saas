package tenantdb

import "testing"

func TestForTenantNilDB(t *testing.T) {
	if ForTenant(1) != nil {
		t.Fatal("expected nil without bootstrap.DB")
	}
	if ForTenant(0) != nil {
		t.Fatal("zero tenant without DB")
	}
}
