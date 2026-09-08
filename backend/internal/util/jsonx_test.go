package util

import "testing"

func TestDecodeJSONObject(t *testing.T) {
	v := DecodeJSON(`{"path":"/pages/index/index","name":"商城首页","type":"shop"}`)
	m, ok := v.(map[string]any)
	if !ok || m["path"] != "/pages/index/index" {
		t.Fatalf("%v", v)
	}
}

func TestToIntString(t *testing.T) {
	if ToInt("1") != 1 || ToInt("01") != 1 {
		t.Fatalf("ToInt string failed")
	}
}

func TestEncodeJSONRoundTrip(t *testing.T) {
	raw := EncodeJSON(map[string]any{"path": "/x"})
	if DecodeJSONMap(raw)["path"] != "/x" {
		t.Fatalf("%s", raw)
	}
}

func TestNoticeTypeDescMatchesPHP(t *testing.T) {
	if NoticeTypeDesc(1) != "业务通知" || NoticeTypeDesc(2) != "验证码" {
		t.Fatalf("known types: %q %q", NoticeTypeDesc(1), NoticeTypeDesc(2))
	}
	if NoticeTypeDesc(0) != "" || NoticeTypeDesc(3) != "" {
		t.Fatalf("unknown types must stay empty, got %q %q", NoticeTypeDesc(0), NoticeTypeDesc(3))
	}
}

func TestSMSStatusDescMatchesPHP(t *testing.T) {
	if SMSStatusDesc("") != "停用" {
		t.Fatalf("empty: %q", SMSStatusDesc(""))
	}
	if SMSStatusDesc(`{"status":1}`) != "启用" {
		t.Fatalf("enabled: %q", SMSStatusDesc(`{"status":1}`))
	}
	if SMSStatusDesc(`{"status":0}`) != "停用" {
		t.Fatalf("disabled: %q", SMSStatusDesc(`{"status":0}`))
	}
	if SMSStatusDesc("not-json") != "停用" {
		t.Fatalf("invalid json: %q", SMSStatusDesc("not-json"))
	}
}
