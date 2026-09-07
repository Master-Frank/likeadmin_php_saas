package cron

import (
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
)

func TestUpdateRefundSuccessWritesTradeNo(t *testing.T) {
	if !initCronDB(t) {
		t.Skip("no database")
	}
	sn := "itrfq" + time.Now().Format("150405.000")
	order := model.RechargeOrder{
		SN: sn, UserID: 1, PayWay: 2, PayStatus: 1, OrderAmount: 1,
		OrderTerminal: 1, TenantID: 1, RefundStatus: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	rec := model.RefundRecord{
		SN: sn + "r", UserID: 1, OrderID: order.ID, OrderSN: sn, OrderType: "recharge",
		OrderAmount: 1, RefundAmount: 1, RefundType: 1, RefundWay: 1, RefundStatus: 0,
		TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	lg := model.RefundLog{
		SN: sn + "l", RecordID: rec.ID, UserID: 1, HandleID: 1,
		OrderAmount: 1, RefundAmount: 1, RefundStatus: 0, TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&lg).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("id = ?", lg.ID).Delete(&model.RefundLog{})
		bootstrap.DB.Where("id = ?", rec.ID).Delete(&model.RefundRecord{})
		bootstrap.DB.Where("id = ?", order.ID).Delete(&model.RechargeOrder{})
	})
	updateRefundSuccess(lg, rec, "503refundid")
	var gotLog model.RefundLog
	var gotRec model.RefundRecord
	var gotOrder model.RechargeOrder
	bootstrap.DB.First(&gotLog, lg.ID)
	bootstrap.DB.First(&gotRec, rec.ID)
	bootstrap.DB.First(&gotOrder, order.ID)
	if gotLog.RefundStatus != 1 || gotRec.RefundStatus != 1 {
		t.Fatalf("status log=%d rec=%d", gotLog.RefundStatus, gotRec.RefundStatus)
	}
	if gotOrder.RefundTransactionID != "503refundid" {
		t.Fatalf("refund_transaction_id=%q", gotOrder.RefundTransactionID)
	}
	if gotLog.RefundMsg != "" {
		t.Fatalf("poll success should not invent refund_msg=%q", gotLog.RefundMsg)
	}
}
