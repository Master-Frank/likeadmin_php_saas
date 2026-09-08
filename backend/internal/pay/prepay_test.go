package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWechatV3NativePrepayFixture(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var gotPath, gotAuth, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code_url":"weixin://wxpay/bizpayurl?pr=fixture"}`))
	}))
	t.Cleanup(srv.Close)
	old := wechatAPIBase
	wechatAPIBase = srv.URL
	t.Cleanup(func() { wechatAPIBase = old })

	cfg := WechatPayCfg{MchID: "1900000001", SerialNo: "SERIAL1"}
	body, _ := json.Marshal(map[string]any{
		"appid": "wx123", "mchid": cfg.MchID, "description": "充值",
		"out_trade_no": "SN202601010001", "notify_url": "http://pair1.likeadmin.test/api/pay/notifyOa",
		"amount": map[string]any{"total": 1050}, "attach": "recharge",
	})
	result, err := wechatV3Post(cfg, key, "/v3/pay/transactions/native", body)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v3/pay/transactions/native" {
		t.Fatalf("path=%s", gotPath)
	}
	if !strings.Contains(gotAuth, `mchid="1900000001"`) || !strings.Contains(gotAuth, `serial_no="SERIAL1"`) {
		t.Fatalf("auth=%s", gotAuth)
	}
	if !strings.Contains(gotBody, `"out_trade_no":"SN202601010001"`) || !strings.Contains(gotBody, `"attach":"recharge"`) {
		t.Fatalf("body=%s", gotBody)
	}
	if result["code_url"] != "weixin://wxpay/bizpayurl?pr=fixture" {
		t.Fatalf("result=%v", result)
	}
}

func TestJSAPIBridgeShapeAndSign(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	got, err := jsapiBridge(key, "wxapp", "prepay123")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"appId", "timeStamp", "nonceStr", "package", "signType", "paySign"} {
		if utilToString(got[k]) == "" {
			t.Fatalf("missing %s: %+v", k, got)
		}
	}
	if got["appId"] != "wxapp" || got["package"] != "prepay_id=prepay123" || got["signType"] != "RSA" {
		t.Fatalf("%+v", got)
	}
	msg := got["appId"].(string) + "\n" + got["timeStamp"].(string) + "\n" + got["nonceStr"].(string) + "\n" + got["package"].(string) + "\n"
	if !verifyRSA2(&key.PublicKey, msg, got["paySign"].(string)) {
		t.Fatal("paySign does not match WeChat JSAPI RSA string")
	}
}

func utilToString(v any) string {
	s, _ := v.(string)
	return s
}

func TestAliFormGatewayAndOrder(t *testing.T) {
	html := aliForm(map[string]string{
		"app_id": "2021000000000000", "method": "alipay.trade.page.pay",
		"biz_content": `{"out_trade_no":"SN1","total_amount":"10.50"}`,
		"sign":        "SIG",
	})
	if !strings.Contains(html, aliGatewayURL+"?charset=utf-8") {
		t.Fatalf("gateway missing: %s", html)
	}
	if !strings.Contains(html, "SN1") || !strings.Contains(html, "10.50") {
		t.Fatalf("order missing: %s", html)
	}
}
