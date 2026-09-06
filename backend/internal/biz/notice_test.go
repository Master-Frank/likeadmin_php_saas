package biz

import (
	"encoding/json"
	"testing"
)

func TestNoticeTypeDesc(t *testing.T) {
	if NoticeTypeDesc(1) != "业务通知" || NoticeTypeDesc(2) != "验证码" {
		t.Fatal(NoticeTypeDesc(1), NoticeTypeDesc(2))
	}
}

func TestFormatNoticeDetail(t *testing.T) {
	if arr, ok := FormatNoticeDetail(0, 2, 101, "", "", "", "", "", "", "").([]any); !ok || len(arr) != 0 {
		t.Fatal("missing id should be empty list")
	}
	raw := `{"type":"sms","template_id":"T1","content":"hi","status":1}`
	out := FormatNoticeDetail(3, 2, 101, "登录验证码", "用户登录", "", raw, "", "", "2,4").(map[string]any)
	if out["type"] != "验证码" {
		t.Fatalf("type=%v", out["type"])
	}
	if out["default"] != "" {
		t.Fatal("default")
	}
	sms := out["sms_notice"].(map[string]any)
	if sms["template_id"] != "T1" || sms["is_show"] != true {
		t.Fatalf("sms=%v", sms)
	}
	tips, _ := sms["tips"].([]string)
	if len(tips) < 2 {
		t.Fatalf("tips=%v", tips)
	}
	sys := out["system_notice"].(map[string]any)
	if sys["is_show"] != false || sys["title"] != "" {
		t.Fatalf("system=%v", sys)
	}
	oa := out["oa_notice"].(map[string]any)
	oaTips, _ := oa["tips"].([]string)
	if len(oaTips) == 0 || oaTips[len(oaTips)-1] != "配置路径：小程序后台 > 功能 > 订阅消息" {
		t.Fatalf("oa tips should follow PHP MNP assignment: %v", oaTips)
	}
}

func TestApplyNoticeSetTemplateObject(t *testing.T) {
	tpl := map[string]any{
		"sms_notice": map[string]any{
			"template_id": "A", "content": "c", "status": 1,
		},
		"system_notice": map[string]any{
			"title": "t", "content": "c", "status": 0,
		},
	}
	updates, err := ApplyNoticeSet(true, 1, tpl)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := updates["sms_notice"]; !ok {
		t.Fatal(updates)
	}
	var sms map[string]any
	if err := json.Unmarshal([]byte(updates["sms_notice"].(string)), &sms); err != nil {
		t.Fatal(err)
	}
	if sms["type"] != "sms" {
		t.Fatalf("inferred type=%v", sms["type"])
	}
}

func TestApplyNoticeSetTemplateList(t *testing.T) {
	tpl := []any{
		map[string]any{"type": "sms", "template_id": "A", "content": "c", "status": 0},
	}
	updates, err := ApplyNoticeSet(true, 2, tpl)
	if err != nil || updates["sms_notice"] == nil {
		t.Fatal(err, updates)
	}
}

func TestApplyNoticeSetErrors(t *testing.T) {
	if _, err := ApplyNoticeSet(false, 1, []any{}); err == nil || err.Error() != "通知配置不存在" {
		t.Fatal(err)
	}
	if _, err := ApplyNoticeSet(true, 1, nil); err == nil || err.Error() != "模板配置不存在或格式错误" {
		t.Fatal(err)
	}
	if _, err := ApplyNoticeSet(true, 1, []any{map[string]any{"type": "sms"}}); err == nil {
		t.Fatal("expected sms required fields")
	}
	if _, err := ApplyNoticeSet(true, 1, []any{map[string]any{"type": "nope"}}); err == nil {
		t.Fatal("expected type error")
	}
}
