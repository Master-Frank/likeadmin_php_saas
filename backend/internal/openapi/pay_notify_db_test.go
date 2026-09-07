package openapi

import (
	"os"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/wechat"
)

func initPayDB(t *testing.T) bool {
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

func TestMarkRechargePaidMovesMoney(t *testing.T) {
	if !initPayDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL").First(&user).Error != nil {
		t.Skip("no tenant user")
	}
	before := user.UserMoney
	beforeTotal := user.TotalRechargeAmount
	sn := "itpay" + time.Now().Format("150405.000")
	order := model.RechargeOrder{
		SN: sn, UserID: user.ID, PayWay: 2, PayStatus: 0, OrderAmount: 3.5,
		OrderTerminal: 1, TenantID: user.TenantID, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("sn = ?", sn).Delete(&model.RechargeOrder{})
		bootstrap.DB.Where("source_sn = ?", sn).Delete(&model.UserAccountLog{})
		bootstrap.DB.Model(&user).Updates(map[string]any{"user_money": before, "total_recharge_amount": beforeTotal})
	})
	if err := markRechargePaid(&order, "wx-itpay"); err != nil {
		t.Fatal(err)
	}
	var got model.RechargeOrder
	bootstrap.DB.Where("id = ?", order.ID).First(&got)
	if got.PayStatus != 1 || got.TransactionID != "wx-itpay" {
		t.Fatalf("order %+v", got)
	}
	var after model.User
	bootstrap.DB.Where("id = ?", user.ID).First(&after)
	if after.UserMoney < before+3.4 {
		t.Fatalf("money %v -> %v", before, after.UserMoney)
	}
	var logs int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("source_sn = ? AND change_type = 201", sn).Count(&logs)
	if logs != 1 {
		t.Fatalf("account logs=%d", logs)
	}
	if err := markRechargePaid(&order, "wx-itpay-2"); err != nil {
		t.Fatal(err)
	}
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("source_sn = ? AND change_type = 201", sn).Count(&logs)
	if logs != 1 {
		t.Fatalf("double pay logs=%d", logs)
	}
}

func TestApplyRefundNotifyMarksLog(t *testing.T) {
	if !initPayDB(t) {
		t.Skip("no database")
	}
	sn := "itrf" + time.Now().Format("150405.000")
	rec := model.RefundRecord{
		SN: sn + "r", UserID: 1, OrderID: 1, OrderSN: sn, OrderType: "recharge",
		OrderAmount: 1, RefundAmount: 1, RefundStatus: 0, TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	lg := model.RefundLog{
		SN: sn, RecordID: rec.ID, UserID: 1, OrderAmount: 1, RefundAmount: 1,
		RefundStatus: 0, TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&lg).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("id = ?", lg.ID).Delete(&model.RefundLog{})
		bootstrap.DB.Where("id = ?", rec.ID).Delete(&model.RefundRecord{})
	})
	pay.ApplyRefundNotify(wechat.PayNotify{RefundOK: true, OutRefundNo: sn})
	var gotLog model.RefundLog
	var gotRec model.RefundRecord
	bootstrap.DB.Where("id = ?", lg.ID).First(&gotLog)
	bootstrap.DB.Where("id = ?", rec.ID).First(&gotRec)
	if gotLog.RefundStatus != 1 || gotRec.RefundStatus != 1 {
		t.Fatalf("log=%d rec=%d", gotLog.RefundStatus, gotRec.RefundStatus)
	}
}
