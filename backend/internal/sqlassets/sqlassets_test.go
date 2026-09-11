package sqlassets

import (
	"strings"
	"testing"
)

func TestEmbeddedSQLPresent(t *testing.T) {
	if !strings.Contains(LikeSQL, "CREATE TABLE `la_dev_crontab`") {
		t.Fatal("like.sql missing crontab table")
	}
	for _, idx := range []string{"idx_sn_delete_time", "idx_domain_alias_delete_time", "idx_type_name", "idx_tenant_type_name", "idx_create_time"} {
		if !strings.Contains(LikeSQL, idx) {
			t.Fatalf("like.sql missing index %s", idx)
		}
	}
	if !strings.Contains(TenantSQL, "idx_tenant_type_name") {
		t.Fatal("tenant.sql missing config index")
	}
	if !strings.Contains(TenantSQL, "CREATE TABLE") {
		t.Fatal("tenant.sql empty")
	}
	if !strings.Contains(TenantDataSQL, "INSERT") && !strings.Contains(TenantDataSQL, "CREATE") {
		t.Fatal("tenantData.sql empty")
	}
}
