package sqlassets

import (
	"strings"
	"testing"
)

func TestEmbeddedSQLPresent(t *testing.T) {
	if !strings.Contains(LikeSQL, "CREATE TABLE `la_dev_crontab`") {
		t.Fatal("like.sql missing crontab table")
	}
	if !strings.Contains(TenantSQL, "CREATE TABLE") {
		t.Fatal("tenant.sql empty")
	}
	if !strings.Contains(TenantDataSQL, "INSERT") && !strings.Contains(TenantDataSQL, "CREATE") {
		t.Fatal("tenantData.sql empty")
	}
}
