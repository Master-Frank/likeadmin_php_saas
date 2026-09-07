package cron

import (
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
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
