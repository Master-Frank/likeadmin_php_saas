package sms

import "testing"

func TestTitleByScene(t *testing.T) {
	oa := map[string]any{"name": "公众号模板"}
	mnp := map[string]any{"name": "小程序模板"}
	if got := titleByScene(sendTypeSMS, oa, mnp); got != "" {
		t.Fatalf("sms title %q", got)
	}
	if got := titleByScene(sendTypeOA, oa, mnp); got != "公众号模板" {
		t.Fatalf("oa title %q", got)
	}
	if got := titleByScene(sendTypeMNP, oa, mnp); got != "小程序模板" {
		t.Fatalf("mnp title %q", got)
	}
	if got := titleByScene(sendTypeOA, map[string]any{}, nil); got != "" {
		t.Fatalf("empty oa %q", got)
	}
}
