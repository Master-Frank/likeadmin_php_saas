package sqlassets

import (
	"strings"
	"testing"
)

func TestEmbeddedSQLPresent(t *testing.T) {
	if !strings.Contains(LikeSQL, "CREATE TABLE `la_dev_crontab`") {
		t.Fatal("like.sql missing crontab table")
	}
	for _, idx := range []string{"idx_sn_delete_time", "idx_domain_alias_delete_time", "idx_type_name", "idx_tenant_type_name", "idx_create_time", "idx_tenant_create_id"} {
		if !strings.Contains(LikeSQL, idx) {
			t.Fatalf("like.sql missing index %s", idx)
		}
	}
	if !strings.Contains(LikeSQL, "`tenant_id`") || !strings.Contains(LikeSQL, "系统日志表") {
		t.Fatal("like.sql missing operation_log tenant_id")
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
