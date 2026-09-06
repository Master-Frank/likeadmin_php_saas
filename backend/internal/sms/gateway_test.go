package sms

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"strings"
	"testing"
)

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
}

func TestTencentParams(t *testing.T) {
	got := tencentParams(map[string]any{"content": "code=${code}"}, "8888", "13800000000")
	if len(got) != 1 || got[0] != "8888" {
		t.Fatalf("%v", got)
	}
}
