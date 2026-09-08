package tenantapi

import (
	"testing"

	"likeadmin/backend/internal/model"
)

func TestPendingRefundLogOmitsMsg(t *testing.T) {
	lg := model.RefundLog{
		SN: "x", RecordID: 1, UserID: 1, HandleID: 1,
		OrderAmount: 1, RefundAmount: 1, RefundStatus: 0, TenantID: 1,
	}
	if lg.RefundMsg != "" {
		t.Fatalf("PHP RefundLogic::log does not set refund_msg, got %q", lg.RefundMsg)
	}
}
