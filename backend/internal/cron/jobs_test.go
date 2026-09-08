package cron

import (
	"os"
	"testing"
	"time"

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

func TestCancelUnpaidSoftDeletesStaleOrders(t *testing.T) {
	if !initCronDB(t) {
		t.Skip("no database")
	}
	now := time.Now().Unix()
	const tid uint = 990010
	stale := model.RechargeOrder{
		SN: "cu-stale-" + time.Now().Format("150405.000"), UserID: 1, PayWay: 2,
		PayStatus: 0, OrderAmount: 1, OrderTerminal: 1, TenantID: tid,
		CreateTime: now - 3600,
	}
	fresh := model.RechargeOrder{
		SN: "cu-fresh-" + time.Now().Format("150405.000"), UserID: 1, PayWay: 2,
		PayStatus: 0, OrderAmount: 1, OrderTerminal: 1, TenantID: tid,
		CreateTime: now - 60,
	}
	paid := model.RechargeOrder{
		SN: "cu-paid-" + time.Now().Format("150405.000"), UserID: 1, PayWay: 2,
		PayStatus: 1, OrderAmount: 1, OrderTerminal: 1, TenantID: tid,
		CreateTime: now - 3600,
	}
	for _, row := range []*model.RechargeOrder{&stale, &fresh, &paid} {
		if err := bootstrap.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("id IN ?", []uint{stale.ID, fresh.ID, paid.ID}).Delete(&model.RechargeOrder{})
	})
	cancelUnpaidForTenant(tid, 1, 30, now)

	var gotStale, gotFresh, gotPaid model.RechargeOrder
	bootstrap.DB.Unscoped().First(&gotStale, stale.ID)
	bootstrap.DB.Unscoped().First(&gotFresh, fresh.ID)
	bootstrap.DB.Unscoped().First(&gotPaid, paid.ID)
	if gotStale.DeleteTime == nil {
		t.Fatal("stale unpaid should be soft-deleted")
	}
	if gotFresh.DeleteTime != nil {
		t.Fatal("fresh unpaid must stay")
	}
	if gotPaid.DeleteTime != nil {
		t.Fatal("paid order must stay")
	}
	cancelUnpaidForTenant(tid, 0, 30, now)
	var again model.RechargeOrder
	bootstrap.DB.Unscoped().First(&again, fresh.ID)
	if again.DeleteTime != nil {
		t.Fatal("disabled setting must not cancel")
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
