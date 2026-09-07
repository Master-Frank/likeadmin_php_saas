package platformapi

import (
	"testing"

	"likeadmin/backend/internal/model"
)

func TestRemapTenantPayConfigID(t *testing.T) {
	oldToNew := map[uint]uint{5: 11, 6: 12}
	wayToID := map[int]uint{1: 11, 2: 12, 3: 13}
	if got := remapTenantPayConfigID(5, oldToNew, wayToID, 1); got != 11 {
		t.Fatalf("old id %d", got)
	}
	if got := remapTenantPayConfigID(2, oldToNew, wayToID, 1); got != 12 {
		t.Fatalf("pay_way match %d", got)
	}
	if got := remapTenantPayConfigID(0, oldToNew, wayToID, 3); got != 13 {
		t.Fatalf("scene fallback %d", got)
	}
	if got := remapTenantPayConfigID(99, oldToNew, wayToID, 1); got != 99 {
		t.Fatalf("keep unknown %d", got)
	}
}

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

func TestCleanTenantScopedRowsSkipsZero(t *testing.T) {
	cleanTenantScopedRows(0)
	expireTenantAdmins(model.Tenant{})
}
