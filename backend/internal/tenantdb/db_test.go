package tenantdb

import "testing"

func TestShardableTables(t *testing.T) {
	need := []string{"user", "article", "tenant_admin", "decorate_tabbar"}
	for _, n := range need {
		if _, ok := shardable[n]; !ok {
			t.Fatalf("missing shard table %s", n)
		}
	}
	if _, ok := shardable["tenant"]; ok {
		t.Fatal("la_tenant itself should not be sharded")
	}
}
