package tenantdb

import (
	"testing"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/model"
)

func TestInvalidateTenantDropsKeys(t *testing.T) {
	cache.Set(hostKey("old.example"), `{"found":true,"id":9,"sn":"abc"}`, 0)
	cache.Set(snKey("abc"), `{"found":true,"id":9,"sn":"abc"}`, 0)
	cache.Set(idKey(9), `{"found":true,"id":9,"sn":"abc"}`, 0)
	cache.Set(hostKey("new.example"), `{"found":true,"id":9}`, 0)
	t.Cleanup(func() {
		cache.Del(hostKey("old.example"))
		cache.Del(hostKey("new.example"))
		cache.Del(snKey("abc"))
		cache.Del(idKey(9))
	})
	InvalidateTenant(
		model.Tenant{ID: 9, SN: "abc", DomainAlias: "old.example"},
		model.Tenant{ID: 9, SN: "abc", DomainAlias: "new.example"},
	)
	for _, key := range []string{hostKey("old.example"), hostKey("new.example"), snKey("abc"), idKey(9)} {
		if _, ok := cache.Get(key); ok {
			t.Fatalf("key still present %s", key)
		}
	}
}

func TestByHostNilDB(t *testing.T) {
	if _, ok := ByHost("missing.example"); ok {
		t.Fatal("nil db must miss")
	}
}
