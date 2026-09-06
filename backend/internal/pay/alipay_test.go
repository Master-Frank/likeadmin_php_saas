package pay

import "testing"

func TestParseAliRefundBodySuccess(t *testing.T) {
	res := ParseAliRefundBody(map[string]any{
		"code": "10000", "msg": "Success", "fund_change": "Y", "trade_no": "20240906001",
	})
	if !res.OK || res.TradeNo != "20240906001" {
		t.Fatalf("%+v", res)
	}
}

func TestParseAliRefundBodyCamelFundChange(t *testing.T) {
	res := ParseAliRefundBody(map[string]any{
		"code": "10000", "msg": "Success", "fundChange": "Y", "tradeNo": "T2",
	})
	if !res.OK || res.TradeNo != "T2" {
		t.Fatalf("%+v", res)
	}
}

func TestParseAliRefundBodyRejects(t *testing.T) {
	if ParseAliRefundBody(nil).OK {
		t.Fatal("nil body")
	}
	if ParseAliRefundBody(map[string]any{"code": "10000", "msg": "Success", "fund_change": "N"}).OK {
		t.Fatal("fund_change N")
	}
	if ParseAliRefundBody(map[string]any{"code": "40004", "msg": "Business Failed", "fund_change": "Y"}).OK {
		t.Fatal("error code")
	}
}

func TestParseAliRefundQuery(t *testing.T) {
	ok, msg, known := ParseAliRefundQuery(nil)
	if ok || known || msg != "" {
		t.Fatal("nil")
	}
	ok, msg, known = ParseAliRefundQuery(map[string]any{"code": "10000", "refund_status": "REFUND_SUCCESS"})
	if !ok || !known || msg != "" {
		t.Fatalf("success %v %s %v", ok, msg, known)
	}
	ok, msg, known = ParseAliRefundQuery(map[string]any{"code": "40004", "msg": "Business Failed", "sub_msg": "订单不存在"})
	if ok || !known || msg != "订单不存在" {
		t.Fatalf("fail %v %s %v", ok, msg, known)
	}
	ok, msg, known = ParseAliRefundQuery(map[string]any{"code": "10000", "refund_status": "REFUND_PROCESSING"})
	if ok || !known || msg != "REFUND_PROCESSING" {
		t.Fatalf("ing %v %s %v", ok, msg, known)
	}
}
