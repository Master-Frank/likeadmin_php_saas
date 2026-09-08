package sms

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAliTemplateParams(t *testing.T) {
	got := aliTemplateParams(LoginCaptcha, map[string]string{"code": "1234", "mobile": "13800000000", "nickname": "n"})
	if len(got) != 1 || got["code"] != "1234" {
		t.Fatalf("captcha %+v", got)
	}
	full := aliTemplateParams(200, map[string]string{"nickname": "n", "order_sn": "SN1", "code": "x"})
	if full["nickname"] != "n" || full["order_sn"] != "SN1" || full["code"] != "x" {
		t.Fatalf("notice %+v", full)
	}
}

func TestAliPercentEncode(t *testing.T) {
	if got := aliPercentEncode("a b*c~"); got != "a%20b%2Ac~" {
		t.Fatalf("got %s", got)
	}
}

func TestAliRPCSignStable(t *testing.T) {
	params := map[string]string{
		"AccessKeyId":      "id",
		"Action":           "SendSms",
		"Format":           "JSON",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"Timestamp":        "2024-01-01T00:00:00Z",
		"Version":          "2017-05-25",
	}
	got := aliRPCSign("secret", params)
	canon := "AccessKeyId=id&Action=SendSms&Format=JSON&SignatureMethod=HMAC-SHA1&SignatureVersion=1.0&Timestamp=2024-01-01T00%3A00%3A00Z&Version=2017-05-25"
	stringToSign := "POST&" + aliPercentEncode("/") + "&" + aliPercentEncode(canon)
	mac := hmac.New(sha1.New, []byte("secret&"))
	mac.Write([]byte(stringToSign))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if got != want {
		t.Fatalf("sign %s want %s", got, want)
	}
}

func TestTencentAuthPrefix(t *testing.T) {
	auth := tencentAuth("AKIDxx", "sk", `{"x":1}`, "1710000000")
	if !strings.HasPrefix(auth, "TC3-HMAC-SHA256 Credential=AKIDxx/") {
		t.Fatalf("auth %s", auth)
	}
	if !strings.Contains(auth, "SignedHeaders=content-type;host") {
		t.Fatalf("missing signed headers: %s", auth)
	}
}

func TestFormatContent(t *testing.T) {
	if got := formatContent("您的验证码是${code}", map[string]string{"code": "1234"}); got != "您的验证码是1234" {
		t.Fatalf("got %s", got)
	}
	if got := formatContent("您好{nickname}", map[string]string{"nickname": "张三"}); got != "您好张三" {
		t.Fatalf("brace %s", got)
	}
	if got := formatContent("", map[string]string{"code": "1234"}); got != "" {
		t.Fatalf("empty template %q", got)
	}
}

func TestTencentParams(t *testing.T) {
	got := tencentParams(map[string]any{"content": "code=${code}"}, map[string]string{
		"code": "8888", "mobile": "13800000000",
	})
	if len(got) != 1 || got[0] != "8888" {
		t.Fatalf("%v", got)
	}
	ordered := tencentParamsFrom("您好${nickname}，验证码${code}", map[string]string{
		"code": "8888", "nickname": "张三", "extra": "x",
	})
	if len(ordered) != 2 || ordered[0] != "张三" || ordered[1] != "8888" {
		t.Fatalf("order %v", ordered)
	}
	notice := tencentParams(map[string]any{"content": "您好${nickname}，单号${order_sn}"}, map[string]string{
		"nickname": "张三", "order_sn": "SN1", "code": "x", "mobile": "13800000000",
	})
	if len(notice) != 2 || notice[0] != "张三" || notice[1] != "SN1" {
		t.Fatalf("full params %v", notice)
	}
}

func TestEncodeSMSResult(t *testing.T) {
	if got := encodeSMSResult("请开启短信配置"); got != `"请开启短信配置"` {
		t.Fatalf("%s", got)
	}
}

func TestGatewayConfigError(t *testing.T) {
	if err := gatewayConfigError("", engineCfg{}, ""); err == nil || err.Error() != "请开启短信配置" {
		t.Fatalf("empty engine: %v", err)
	}
	if err := gatewayConfigError("foo", engineCfg{}, "T"); err == nil || err.Error() != "没有相应的短信驱动类" {
		t.Fatalf("unknown: %v", err)
	}
	if err := gatewayConfigError("ali", engineCfg{Status: 1}, "T"); err == nil || err.Error() != "ali未配置" {
		t.Fatalf("incomplete: %v", err)
	}
	if err := gatewayConfigError("ali", engineCfg{Status: 0, AppKey: "a", SecretKey: "s", Sign: "n"}, "T"); err == nil || err.Error() != "短信服务未开启" {
		t.Fatalf("disabled: %v", err)
	}
	if err := gatewayConfigError("ali", engineCfg{Status: 1, AppKey: "a", SecretKey: "s", Sign: "n"}, ""); err == nil || err.Error() != "短信服务未开启" {
		t.Fatalf("no tpl: %v", err)
	}
	if err := gatewayConfigError("ali", engineCfg{Status: 1, AppKey: "a", SecretKey: "s", Sign: "n"}, "SMS_1"); err != nil {
		t.Fatalf("ready: %v", err)
	}
}

func TestSendAliyunFixture(t *testing.T) {
	var phone, sign, tpl, signature string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		phone, sign, tpl, signature = r.Form.Get("PhoneNumbers"), r.Form.Get("SignName"), r.Form.Get("TemplateCode"), r.Form.Get("Signature")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"Code":"OK","Message":"OK","BizId":"1"}`)
	}))
	t.Cleanup(srv.Close)
	old := aliSMSURL
	aliSMSURL = srv.URL
	t.Cleanup(func() { aliSMSURL = old })
	got, err := sendAliyun(engineCfg{AppKey: "ak", SecretKey: "sk", Sign: "likeadmin"}, "13800000000", "SMS_123", map[string]string{"code": "8888"})
	if err != nil {
		t.Fatal(err)
	}
	if phone != "13800000000" || sign != "likeadmin" || tpl != "SMS_123" || signature == "" {
		t.Fatalf("form %s %s %s sig=%s", phone, sign, tpl, signature)
	}
	m, _ := got.(map[string]any)
	if m["Code"] != "OK" {
		t.Fatalf("%v", got)
	}
}

func TestSendTencentFixture(t *testing.T) {
	var action, auth, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		action = r.Header.Get("X-TC-Action")
		auth = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"Response":{"SendStatusSet":[{"Code":"Ok"}]}}`)
	}))
	t.Cleanup(srv.Close)
	old := tencentSMSURL
	tencentSMSURL = srv.URL
	t.Cleanup(func() { tencentSMSURL = old })
	got, err := sendTencent(engineCfg{SecretID: "id", SecretKey: "sk", Sign: "likeadmin", AppID: "1400000000"}, "13800000000", "123456", []string{"8888"})
	if err != nil {
		t.Fatal(err)
	}
	if action != "SendSms" || !strings.HasPrefix(auth, "TC3-HMAC-SHA256 ") {
		t.Fatalf("action=%s auth=%s", action, auth)
	}
	if !strings.Contains(body, `"TemplateID":"123456"`) || !strings.Contains(body, "+8613800000000") {
		t.Fatalf("body=%s", body)
	}
	if got == nil {
		t.Fatal("empty result")
	}
}
