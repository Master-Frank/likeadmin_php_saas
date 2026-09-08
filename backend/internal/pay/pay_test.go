package pay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"os"
	"testing"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/wechat"
)

func TestRSASignVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := rsaSHA256Base64(key, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyRSA2(&key.PublicKey, "hello", sig) {
		t.Fatal("verify failed")
	}
	if verifyRSA2(&key.PublicKey, "other", sig) {
		t.Fatal("bad message accepted")
	}
}

func TestParseRSAPrivateKeyPKCS1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	got, err := parseRSAPrivateKey(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.N.Cmp(key.N) != 0 {
		t.Fatal("key mismatch")
	}
}

func TestAliSignContent(t *testing.T) {
	got := aliSignContent(map[string]string{"b": "2", "a": "1", "sign": "x", "empty": ""})
	if got != "a=1&b=2" {
		t.Fatalf("got %q", got)
	}
}

func TestDecryptWechatV3(t *testing.T) {
	key := "12345678901234567890123456789012"
	plain := []byte(`{"out_trade_no":"SN1234567890123456","transaction_id":"wx1","trade_state":"SUCCESS","attach":"recharge"}`)
	nonce := "123456789012"
	aad := "transaction"
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte(nonce), plain, []byte(aad))
	env := map[string]any{
		"event_type": "TRANSACTION.SUCCESS",
		"resource": map[string]any{
			"ciphertext":      base64.StdEncoding.EncodeToString(ct),
			"associated_data": aad,
			"nonce":           nonce,
		},
	}
	raw, _ := json.Marshal(env)
	n := DecryptWechatV3(raw, key)
	if !n.Paid || n.OutTradeNo != "SN1234567890123456" || n.Attach != "recharge" || n.TransactionID != "wx1" {
		t.Fatalf("%+v", n)
	}
	if _, ok := DecryptWechatV3OK(raw, "wrong-key-wrong-key-wrong-key!!"); ok {
		t.Fatal("bad key accepted")
	}
	if _, ok := DecryptWechatV3OK([]byte(`{"resource":{"ciphertext":"xxxx"}}`), ""); ok {
		t.Fatal("empty key accepted")
	}
	if n, ok := DecryptWechatV3WithKeys(raw, []string{"", "wrong-key-wrong-key-wrong-key!!", key}); !ok || n.OutTradeNo != "SN1234567890123456" {
		t.Fatalf("fallback %+v ok=%v", n, ok)
	}
	if _, ok := DecryptWechatV3WithKeys(raw, []string{"", "wrong-key-wrong-key-wrong-key!!"}); ok {
		t.Fatal("all bad keys accepted")
	}
	plainV3 := []byte(`{"event_type":"REFUND.SUCCESS","out_refund_no":"RF1","refund_status":"SUCCESS"}`)
	n, ok := DecryptWechatV3WithKeys(plainV3, nil)
	if !ok || n.OutRefundNo != "RF1" || !n.RefundOK {
		t.Fatalf("plain v3 %+v ok=%v", n, ok)
	}
}

func TestDecryptWechatV3RefundSuccess(t *testing.T) {
	key := "12345678901234567890123456789012"
	plain := []byte(`{"out_refund_no":"RF1788810001","refund_id":"501refund","refund_status":"SUCCESS","transaction_id":"wx-rf-1"}`)
	nonce := "refundnonce1"
	aad := "refund"
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte(nonce), plain, []byte(aad))
	env := map[string]any{
		"event_type": "REFUND.SUCCESS",
		"resource": map[string]any{
			"ciphertext":      base64.StdEncoding.EncodeToString(ct),
			"associated_data": aad,
			"nonce":           nonce,
		},
	}
	raw, _ := json.Marshal(env)
	n, ok := DecryptWechatV3OK(raw, key)
	if !ok || !n.RefundOK || n.OutRefundNo != "RF1788810001" || n.TransactionID != "wx-rf-1" {
		t.Fatalf("encrypted refund %+v ok=%v", n, ok)
	}
	if n.EventType != "REFUND.SUCCESS" {
		t.Fatalf("event %s", n.EventType)
	}
}

func TestNormalizePEM(t *testing.T) {
	got := normalizePEM("abcd", "PUBLIC KEY")
	if got == "" || got[:10] != "-----BEGIN" {
		t.Fatalf("pem wrap failed: %s", got)
	}
}

func TestWechatRefundMissingConfig(t *testing.T) {
	err := WechatRefund(nil, "", "rf1", 1, 1)
	if err == nil || err.Error() != "请先完成支付渠道配置" {
		t.Fatalf("cfg %v", err)
	}
}

func TestAliPayWay(t *testing.T) {
	method, product, err := aliPayWay(wechat.TerminalPC)
	if err != nil || method != "alipay.trade.page.pay" || product != "FAST_INSTANT_TRADE_PAY" {
		t.Fatalf("pc %s %s %v", method, product, err)
	}
	method, product, err = aliPayWay(wechat.TerminalH5)
	if err != nil || method != "alipay.trade.wap.pay" || product != "QUICK_WAP_WAY" {
		t.Fatalf("h5 %s %s %v", method, product, err)
	}
	method, product, err = aliPayWay(wechat.TerminalIOS)
	if err != nil || method != "alipay.trade.app.pay" || product != "QUICK_MSECURITY_PAY" {
		t.Fatalf("ios %s %s %v", method, product, err)
	}
	if _, _, err := aliPayWay(0); err == nil || err.Error() != "支付方式错误" {
		t.Fatalf("unknown 0: %v", err)
	}
	if _, _, err := aliPayWay(wechat.TerminalMNP); err == nil || err.Error() != "支付方式错误" {
		t.Fatalf("mnp: %v", err)
	}
}

func TestAliPrepayMissingConfig(t *testing.T) {
	_, err := AliPrepay(nil, model.RechargeOrder{OrderAmount: 1}, "recharge", "/", wechat.TerminalOA)
	if err == nil || err.Error() != "请配置好支付设置" {
		t.Fatalf("ali prepay cfg %v", err)
	}
}

func TestDebugPayOverride(t *testing.T) {
	oldDebug := config.C.App.Debug
	t.Cleanup(func() {
		config.C.App.Debug = oldDebug
		_ = os.Unsetenv("LIKEADMIN_TEST_WEB_IP")
	})
	config.C.App.Debug = false
	t.Setenv("LIKEADMIN_TEST_WEB_IP", "9.9.9.9")
	if got := debugPayOverride("LIKEADMIN_TEST_WEB_IP", "1.1.1.1"); got != "1.1.1.1" {
		t.Fatalf("debug off: %s", got)
	}
	config.C.App.Debug = true
	if got := debugPayOverride("LIKEADMIN_TEST_WEB_IP", "1.1.1.1"); got != "9.9.9.9" {
		t.Fatalf("debug on: %s", got)
	}
}

func TestYuanToFenMatchesPHP(t *testing.T) {
	// PHP: 19.9*100 intval=1989; intval(strval)=1990 (H5 mwebPay).
	if got := yuanToFenIntval(19.9); got != 1989 {
		t.Fatalf("intval 19.9 => %d", got)
	}
	if got := yuanToFenStrval(19.9); got != 1990 {
		t.Fatalf("strval 19.9 => %d", got)
	}
	if got := yuanToFenIntval(1.15); got != 114 {
		t.Fatalf("intval 1.15 => %d", got)
	}
	if got := yuanToFenStrval(1.15); got != 115 {
		t.Fatalf("strval 1.15 => %d", got)
	}
	if got := yuanToFen(19.9, wechat.TerminalPC); got != 1989 {
		t.Fatalf("native 19.9 => %d", got)
	}
	if got := yuanToFen(19.9, wechat.TerminalH5); got != 1990 {
		t.Fatalf("h5 19.9 => %d", got)
	}
	if got := yuanToFenIntval(100); got != 10000 {
		t.Fatalf("100 => %d", got)
	}
}

func TestWechatPayPath(t *testing.T) {
	cases := map[int]string{
		wechat.TerminalMNP:     "/v3/pay/transactions/jsapi",
		wechat.TerminalOA:      "/v3/pay/transactions/jsapi",
		wechat.TerminalH5:      "/v3/pay/transactions/h5",
		wechat.TerminalIOS:     "/v3/pay/transactions/app",
		wechat.TerminalAndroid: "/v3/pay/transactions/app",
		wechat.TerminalPC:      "/v3/pay/transactions/native",
	}
	for term, want := range cases {
		got, err := wechatPayPath(term)
		if err != nil || got != want {
			t.Fatalf("term=%d path=%s err=%v want=%s", term, got, err, want)
		}
	}
	if _, err := wechatPayPath(0); err == nil || err.Error() != "支付方式错误" {
		t.Fatalf("unknown 0: %v", err)
	}
	if _, err := wechatPayPath(99); err == nil || err.Error() != "支付方式错误" {
		t.Fatalf("unknown 99: %v", err)
	}
}

func TestWechatChannelMissing(t *testing.T) {
	if wechatChannelMissing(wechat.TerminalMNP) != "请先设置小程序配置" {
		t.Fatal(wechatChannelMissing(wechat.TerminalMNP))
	}
	if wechatChannelMissing(wechat.TerminalOA) != "请先设置公众号配置" {
		t.Fatal(wechatChannelMissing(wechat.TerminalOA))
	}
}

func TestAliRefundMissingConfig(t *testing.T) {
	_, err := AliRefundByTenant(0, "sn1", "rf1", 1)
	if err == nil || err.Error() != "请配置好支付设置" {
		t.Fatalf("cfg %v", err)
	}
}

func TestParseWechatRefundQuery(t *testing.T) {
	ok, msg, known := ParseWechatRefundQuery(nil)
	if ok || known || msg != "" {
		t.Fatalf("nil %+v %q %v", ok, msg, known)
	}
	ok, msg, known = ParseWechatRefundQuery(map[string]any{"status": "SUCCESS"})
	if !ok || !known || msg != "" {
		t.Fatalf("success %+v %q %v", ok, msg, known)
	}
	ok, msg, known = ParseWechatRefundQuery(map[string]any{"code": "PARAM_ERROR", "message": "bad"})
	if ok || !known || msg != "PARAM_ERROR-bad" {
		t.Fatalf("err %+v %q %v", ok, msg, known)
	}
	ok, msg, known = ParseWechatRefundQuery(map[string]any{"status": "PROCESSING"})
	if ok || known || msg != "" {
		t.Fatalf("processing %+v %q %v", ok, msg, known)
	}
}

func TestWechatResultFail(t *testing.T) {
	if err := wechatResultFail(nil); err != nil {
		t.Fatal(err)
	}
	if err := wechatResultFail(map[string]any{"status": "SUCCESS"}); err != nil {
		t.Fatal(err)
	}
	err := wechatResultFail(map[string]any{"code": "PARAM_ERROR", "message": "bad"})
	if err == nil || err.Error() != "微信:PARAM_ERROR-bad" {
		t.Fatalf("%v", err)
	}
	err = wechatResultFail(map[string]any{"message": "only"})
	if err == nil || err.Error() != "微信:-only" {
		t.Fatalf("%v", err)
	}
}

func TestRefundQueryTradeNo(t *testing.T) {
	if got := RefundQueryTradeNo(nil); got != "" {
		t.Fatal(got)
	}
	if got := RefundQueryTradeNo(map[string]any{"refund_id": "503xxx", "transaction_id": "420xxx"}); got != "503xxx" {
		t.Fatalf("wechat %s", got)
	}
	if got := RefundQueryTradeNo(map[string]any{"trade_no": "20240906"}); got != "20240906" {
		t.Fatalf("ali %s", got)
	}
	if got := RefundQueryTradeNo(map[string]any{"tradeNo": "T2"}); got != "T2" {
		t.Fatalf("camel %s", got)
	}
}
