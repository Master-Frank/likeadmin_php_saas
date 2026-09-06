package biz

import (
	"testing"
)

func TestCheckPayConfigBalance(t *testing.T) {
	in := PayConfigInput{
		ID: 1, Name: "余额支付", Icon: "/icon.png", Sort: 1, SortPresent: true,
		PayWay: PayBalance, Exists: true,
	}
	if msg := CheckPayConfig(in); msg != "" {
		t.Fatal(msg)
	}
}

func TestCheckPayConfigWechat(t *testing.T) {
	in := PayConfigInput{
		ID: 2, Name: "微信支付", Icon: "/i.png", Sort: 2, SortPresent: true,
		PayWay: PayWechat, Exists: true, ConfigPresent: true,
		Config: map[string]any{},
	}
	if msg := CheckPayConfig(in); msg != "微信支付接口版本不能为空" {
		t.Fatal(msg)
	}
	in.Config = map[string]any{
		"interface_version": "v3", "merchant_type": "ordinary_merchant",
		"mch_id": "1", "pay_sign_key": "k", "apiclient_cert": "c", "apiclient_key": "k",
	}
	if msg := CheckPayConfig(in); msg != "" {
		t.Fatal(msg)
	}
}

func TestCheckPayConfigMessages(t *testing.T) {
	if CheckPayConfig(PayConfigInput{}) != "id不能为空" {
		t.Fatal(CheckPayConfig(PayConfigInput{}))
	}
	in := PayConfigInput{ID: 1, Exists: true}
	if CheckPayConfig(in) != "支付名称不能为空" {
		t.Fatal(CheckPayConfig(in))
	}
	in.Name = "n"
	in.NameTaken = true
	if CheckPayConfig(in) != "支付名称已存在" {
		t.Fatal(CheckPayConfig(in))
	}
	in.NameTaken = false
	if CheckPayConfig(in) != "支付图标不能为空" {
		t.Fatal(CheckPayConfig(in))
	}
	in.Icon = "i"
	if CheckPayConfig(in) != "排序不能为空" {
		t.Fatal(CheckPayConfig(in))
	}
	in.SortPresent = true
	in.Sort = "x"
	if CheckPayConfig(in) != "排序必须是纯数字" {
		t.Fatal(CheckPayConfig(in))
	}
}

func TestCheckPayConfigAlipayCertificate(t *testing.T) {
	in := PayConfigInput{
		ID: 3, Name: "支付宝", Icon: "/i.png", Sort: 3, SortPresent: true,
		PayWay: PayAlipay, Exists: true, ConfigPresent: true,
		Config: map[string]any{
			"mode": "certificate", "merchant_type": "ordinary_merchant",
			"app_id": "a", "private_key": "k",
		},
	}
	if msg := CheckPayConfig(in); msg != "应用公钥证书不能为空" {
		t.Fatal(msg)
	}
	in.Config = map[string]any{
		"mode": "certificate", "merchant_type": "ordinary_merchant",
		"app_id": "a", "private_key": "k", "public_cert": "c",
	}
	if msg := CheckPayConfig(in); msg != "支付宝公钥证书不能为空" {
		t.Fatal(msg)
	}
	in.Config = map[string]any{
		"mode": "certificate", "merchant_type": "ordinary_merchant",
		"app_id": "a", "private_key": "k", "public_cert": "c", "ali_public_cert": "p",
	}
	if msg := CheckPayConfig(in); msg != "支付宝根证书不能为空" {
		t.Fatal(msg)
	}
	in.Config = map[string]any{
		"mode": "certificate", "merchant_type": "ordinary_merchant",
		"app_id": "a", "private_key": "k", "public_cert": "c",
		"ali_public_cert": "p", "ali_root_cert": "r",
	}
	if msg := CheckPayConfig(in); msg != "" {
		t.Fatal(msg)
	}
	in.Config = map[string]any{
		"mode": "normal_mode", "merchant_type": "ordinary_merchant",
		"app_id": "a", "private_key": "k",
	}
	if msg := CheckPayConfig(in); msg != "支付宝公钥不能为空" {
		t.Fatal(msg)
	}
}

func TestBuildPayConfigJSON(t *testing.T) {
	if BuildPayConfigJSON(PayBalance, map[string]any{"x": 1}) != "" {
		t.Fatal("balance config should be empty string")
	}
	raw := BuildPayConfigJSON(PayWechat, map[string]any{
		"interface_version": "v3", "merchant_type": "ordinary_merchant",
		"mch_id": "m", "pay_sign_key": "k", "apiclient_cert": "c", "apiclient_key": "p",
		"extra": "drop",
	})
	if raw == "" || DecodePayConfig(raw).(map[string]any)["mch_id"] != "m" {
		t.Fatal(raw)
	}
	if _, ok := DecodePayConfig(raw).(map[string]any)["extra"]; ok {
		t.Fatal("extra key should be dropped")
	}
}
