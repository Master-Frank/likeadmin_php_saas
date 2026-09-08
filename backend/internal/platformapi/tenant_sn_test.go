package platformapi

import (
	"encoding/json"
	"testing"

	"likeadmin/backend/internal/config"
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

func TestApplyTenantSQLPlaceholders(t *testing.T) {
	oldPrefix := config.C.Database.Prefix
	oldDB := config.C.Database.Database
	config.C.Database.Prefix = "lk_"
	config.C.Database.Database = "testdb"
	defer func() {
		config.C.Database.Prefix = oldPrefix
		config.C.Database.Database = oldDB
	}()
	got := applyTenantSQLPlaceholders("INSERT INTO `la_article_{tenantSn}` VALUES ({tenantId});", "pair2", 9)
	if got != "INSERT INTO testdb.`lk_article_pair2` VALUES (9);" {
		t.Fatalf("%s", got)
	}
}

func TestTenantSQLDatabaseDefault(t *testing.T) {
	old := config.C.Database.Database
	config.C.Database.Database = ""
	defer func() { config.C.Database.Database = old }()
	if tenantSQLDatabase() != "likeadmin_saas" {
		t.Fatalf("%s", tenantSQLDatabase())
	}
}

func TestCanonicalizeNoticeJSON(t *testing.T) {
	raw := canonicalizeNoticeJSON(`{"status":1,"content":"hi"}`)
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil || int(m["status"].(float64)) != 1 {
		t.Fatalf("%s", raw)
	}
	dbl := canonicalizeNoticeJSON(`"{\"status\":0}"`)
	if json.Unmarshal([]byte(dbl), &m) != nil {
		t.Fatalf("double %s", dbl)
	}
}

func TestNewTenantSuperAdminStampsTimes(t *testing.T) {
	now := int64(1700000000)
	admin := newTenantSuperAdmin(0, 9, "pair9", "hash", now)
	if admin.ID != 0 || admin.TenantID != 9 || admin.Account != "pair9" || admin.Name != "超级管理员" {
		t.Fatalf("%+v", admin)
	}
	if admin.Root != 1 || admin.MultipointLogin != 1 || admin.CreateTime != now {
		t.Fatalf("fields %+v", admin)
	}
	if admin.UpdateTime == nil || *admin.UpdateTime != now {
		t.Fatalf("update_time %+v", admin.UpdateTime)
	}
	sharded := newTenantSuperAdmin(1, 9, "admin", "hash", now)
	if sharded.ID != 1 || sharded.UpdateTime == nil || *sharded.UpdateTime != now {
		t.Fatalf("sharded %+v", sharded)
	}
}

func TestTenantAliasTakenSkipsEmpty(t *testing.T) {
	if tenantAliasTaken("", 0) {
		t.Fatal("empty alias must skip uniqueness like PHP TenantValidate")
	}
	if tenantAliasTaken("", 7) {
		t.Fatal("empty alias with exclude must still skip")
	}
}
