package openapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlePayNotifyRejectsBadSign(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/pay/notifyOa", handlePayNotify)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/pay/notifyOa", strings.NewReader(
		"<xml><out_trade_no>SN1</out_trade_no><result_code>SUCCESS</result_code><sign>bad</sign></xml>"))
	req.Header.Set("Content-Type", "application/xml")
	r.ServeHTTP(rec, req)
	if rec.Body.String() != "fail" {
		t.Fatalf("v2 unsigned: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/pay/notifyOa", bytes.NewReader(
		[]byte(`{"event_type":"TRANSACTION.SUCCESS","resource":{"ciphertext":"x","nonce":"n","associated_data":"transaction"}}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":"FAIL"`) || !strings.Contains(rec.Body.String(), "解密失败") {
		t.Fatalf("v3 decrypt: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/pay/notifyOa", bytes.NewReader(
		[]byte(`{"event_type":"TRANSACTION.SUCCESS","resource":{"ciphertext":"x","nonce":"n","associated_data":"transaction"}}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Wechatpay-Signature", "bad")
	req.Header.Set("Wechatpay-Timestamp", "1")
	req.Header.Set("Wechatpay-Nonce", "n")
	r.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "验签失败") {
		t.Fatalf("v3 bad sign: %s", rec.Body.String())
	}
}

func TestHandlePayNotifyEmptyIsSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/pay/aliNotify", strings.NewReader(""))
	handlePayNotify(c)
	if w.Body.String() != "success" {
		t.Fatalf("empty: %s", w.Body.String())
	}
}
