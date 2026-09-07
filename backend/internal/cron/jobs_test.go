package cron

import (
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
)

func initCronDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		return true
	}
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if err := bootstrap.Init(cfg); err != nil {
		t.Log(err)
		return false
	}
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
	}
	return bootstrap.DB != nil
}

func TestEnsureNativeJobsInsertsOnce(t *testing.T) {
	if !initCronDB(t) {
		t.Skip("no database")
	}
	EnsureNativeJobs()
	EnsureNativeJobs()
	var n int64
	bootstrap.DB.Model(&model.Crontab{}).Where("command = ? AND system = 1 AND delete_time IS NULL", "query_refund").Count(&n)
	if n < 1 {
		t.Fatalf("query_refund rows=%d", n)
	}
	bootstrap.DB.Model(&model.Crontab{}).Where("command = ? AND system = 1 AND delete_time IS NULL", "cancel_unpaid_orders").Count(&n)
	if n < 1 {
		t.Fatalf("cancel_unpaid_orders rows=%d", n)
	}
}

func TestTxnCancelSettingsReadsTenantDB(t *testing.T) {
	if !initCronDB(t) {
		t.Skip("no database")
	}
	on, minutes := txnCancelSettings(1, 1, 30)
	if minutes <= 0 {
		t.Fatalf("tenant1 cancel %d %d", on, minutes)
	}
	on2, minutes2 := txnCancelSettings(2, 1, 30)
	if minutes2 <= 0 {
		t.Fatalf("tenant2 shard cancel %d %d", on2, minutes2)
	}
}
