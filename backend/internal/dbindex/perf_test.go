package dbindex

import (
	"testing"

	"gorm.io/gorm"
)

func TestEnsurePerfIndexesNilDB(t *testing.T) {
	EnsurePerfIndexes(nil)
}

func TestEnsurePerfIndexesSkipsWhenDDLDisabled(t *testing.T) {
	t.Setenv("LIKEADMIN_REQUIRE_DDL", "0")
	EnsurePerfIndexes(&gorm.DB{})
}

func TestIsShardCopy(t *testing.T) {
	if !isShardCopy("la_article_ab12", "la_article") {
		t.Fatal("shard copy")
	}
	if isShardCopy("la_article_cate", "la_article") {
		t.Fatal("article_cate is not article shard")
	}
	if isShardCopy("la_user_account_log", "la_user") {
		t.Fatal("account_log is not user shard")
	}
	if !isShardCopy("la_tenant_config_ab12", "la_tenant_config") {
		t.Fatal("tenant_config shard")
	}
}
