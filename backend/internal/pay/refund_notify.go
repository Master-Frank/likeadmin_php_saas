package pay

import (
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"
)

// ApplyRefundNotify marks a pending refund log/record paid when WeChat posts REFUND.SUCCESS.
func ApplyRefundNotify(n wechat.PayNotify) {
	if !wechat.ShouldApplyRefund(n) || bootstrap.DB == nil {
		return
	}
	var lg model.RefundLog
	if bootstrap.DB.Where("sn = ?", n.OutRefundNo).First(&lg).Error != nil {
		return
	}
	if lg.RefundStatus == 1 {
		return
	}
	var rec model.RefundRecord
	rq := bootstrap.DB.Where("id = ?", lg.RecordID)
	if lg.TenantID > 0 {
		rq = rq.Where("tenant_id = ?", lg.TenantID)
	}
	if rq.First(&rec).Error != nil {
		return
	}
	now := util.NowUnix()
	lq := bootstrap.DB.Model(&model.RefundLog{}).Where("id = ? AND refund_status = 0", lg.ID)
	if lg.TenantID > 0 {
		lq = lq.Where("tenant_id = ?", lg.TenantID)
	}
	lq.Updates(map[string]any{"refund_status": 1, "update_time": now})
	uq := bootstrap.DB.Model(&model.RefundRecord{}).Where("id = ?", rec.ID)
	if rec.TenantID > 0 {
		uq = uq.Where("tenant_id = ?", rec.TenantID)
	}
	uq.Updates(map[string]any{"refund_status": 1, "update_time": now})
	if rec.OrderType == "recharge" && rec.OrderID > 0 && n.TransactionID != "" {
		oq := bootstrap.DB.Model(&model.RechargeOrder{}).Where("id = ?", rec.OrderID)
		if rec.TenantID > 0 {
			oq = oq.Where("tenant_id = ?", rec.TenantID)
		}
		oq.Updates(map[string]any{"refund_transaction_id": n.TransactionID, "update_time": now})
	}
}
