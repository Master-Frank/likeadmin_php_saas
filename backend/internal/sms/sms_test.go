package sms

import (
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

func TestSceneByTag(t *testing.T) {
	cases := map[string]int{
		"YZMDL":               LoginCaptcha,
		"yzmdl":               LoginCaptcha,
		"BDSJHM":              BindMobileCaptcha,
		"BGSJHM":              ChangeMobileCaptcha,
		"ZHDLMM":              FindPasswordCaptcha,
		"bind_mobile":         BindMobileCaptcha,
		"change_mobile":       ChangeMobileCaptcha,
		"find_login_password": FindPasswordCaptcha,
		"101":                 LoginCaptcha,
		"":                    0,
		"unknown":             0,
	}
	for in, want := range cases {
		if got := SceneByTag(in); got != want {
			t.Fatalf("SceneByTag(%q)=%d want %d", in, got, want)
		}
	}
}

func TestSendVerifyWithoutDB(t *testing.T) {
	_, code, err := Send(nil, "13800000000", "YZMDL")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 4 {
		t.Fatalf("code len %d", len(code))
	}
	if !Verify(nil, "13800000000", code, "YZMDL") {
		t.Fatal("verify tag failed")
	}
	if Verify(nil, "13800000000", code, "YZMDL") {
		t.Fatal("code should be consumed")
	}
}

func TestPlatformSMS(t *testing.T) {
	if platformSMS(nil) {
		t.Fatal("nil context is tenant/user")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if platformSMS(c) {
		t.Fatal("empty meta is not platform")
	}
	meta := ctxutil.Get(c)
	meta.App = "platformapi"
	meta.Source = ctxutil.SourcePlatform
	if !platformSMS(c) {
		t.Fatal("platformapi should use la_sms_log")
	}
	meta.App = "api"
	meta.Source = ctxutil.SourceUser
	if platformSMS(c) {
		t.Fatal("user api should use la_tenant_sms_log")
	}
	meta.App = "tenantapi"
	meta.Source = ctxutil.SourceTenant
	if platformSMS(c) {
		t.Fatal("tenantapi should use la_tenant_sms_log")
	}
}

func TestMergeNoticeParamsEmpty(t *testing.T) {
	got := mergeNoticeParams(nil, map[string]string{"code": "1234", "mobile": "13800000000"})
	if got["code"] != "1234" || got["mobile"] != "13800000000" {
		t.Fatalf("%+v", got)
	}
	if got["nickname"] != "" {
		t.Fatalf("no user should not enrich %+v", got)
	}
}

func TestNoticeBySceneMissing(t *testing.T) {
	if err := NoticeByScene(nil, 0, nil); err == nil || err.Error() != "找不到对应场景的配置" {
		t.Fatalf("%v", err)
	}
	if err := NoticeByScene(nil, 101, nil); err == nil || err.Error() != "找不到对应场景的配置" {
		t.Fatalf("%v", err)
	}
}
