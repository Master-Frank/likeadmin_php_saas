package platformapi

import (
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
)

func TestReadTenantSQLEmbedFallback(t *testing.T) {
	old := config.C.App.PublicDir
	t.Cleanup(func() { config.C.App.PublicDir = old })
	config.C.App.PublicDir = t.TempDir()

	raw, err := readTenantSQL("tenant.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "CREATE TABLE") {
		t.Fatal("tenant.sql embed empty")
	}
	data, err := readTenantSQL("tenantData.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("tenantData.sql embed empty")
	}
}
