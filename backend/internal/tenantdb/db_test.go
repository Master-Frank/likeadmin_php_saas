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

func TestWithSNNilAndEmpty(t *testing.T) {
	if WithSN(nil, "pair2") != nil {
		t.Fatal("nil db")
	}
	if ForTenantOn(nil, 2) != nil {
		t.Fatal("nil ForTenantOn")
	}
}

func TestUseReadWithoutReplica(t *testing.T) {
	if UseRead(nil) != nil {
		t.Fatal("nil bootstrap.Read must stay nil")
	}
}
