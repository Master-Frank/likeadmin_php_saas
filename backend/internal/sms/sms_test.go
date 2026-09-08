package sms

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func initSMSDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		return true
	}
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if err := bootstrap.Init(cfg); err != nil {
		t.Log(err)
		return false
	}
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
	}
	return bootstrap.DB != nil
}

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
		"999":                 0,
	}
	for in, want := range cases {
		if got := SceneByTag(in); got != want {
			t.Fatalf("SceneByTag(%q)=%d want %d", in, got, want)
		}
	}
}

func TestVerifyIgnoresCacheWhenDBMiss(t *testing.T) {
	if !initSMSDB(t) {
		t.Skip("no database")
	}
	mobile := fmt.Sprintf("136%08d", time.Now().UnixNano()%100000000)
	cache.Set(cacheKey(LoginCaptcha, mobile), "9999", time.Minute)
	t.Cleanup(func() { cache.Del(cacheKey(LoginCaptcha, mobile)) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceUser, TenantID: 1, App: "api"})
	if Verify(c, mobile, "9999", "YZMDL") {
		t.Fatal("PHP SmsDriver::verify must not accept a cache-only code when DB is up")
	}
}

func TestVerifyStampsUpdateTime(t *testing.T) {
	if !initSMSDB(t) {
		t.Skip("no database")
	}
	mobile := fmt.Sprintf("135%08d", time.Now().UnixNano()%100000000)
	old := time.Now().Unix() - 90
	now := time.Now().Unix()
	row := model.TenantSmsLog{
		SceneID: LoginCaptcha, Mobile: mobile, Code: "4321", Content: "code",
		IsVerify: 0, CheckNum: 0, SendStatus: 1, SendTime: util.UnixPtr(now),
		TenantID: 1, CreateTime: old, UpdateTime: util.UnixPtr(old),
	}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.TenantSmsLog{}) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceUser, TenantID: 1, App: "api"})
	if !Verify(c, mobile, "4321", "YZMDL") {
		t.Fatal("verify should succeed")
	}
	var got model.TenantSmsLog
	bootstrap.DB.First(&got, row.ID)
	if got.IsVerify != 1 || got.CheckNum != 1 {
		t.Fatalf("verify=%d check=%d", got.IsVerify, got.CheckNum)
	}
	if got.UpdateTime == nil || *got.UpdateTime <= old {
		t.Fatalf("update_time=%v want > %d", got.UpdateTime, old)
	}
}

func TestSendRateLimitWritesFailLog(t *testing.T) {
	if !initSMSDB(t) {
		t.Skip("no database")
	}
	var setting model.NoticeSetting
	if bootstrap.DB.Where("scene_id = ?", LoginCaptcha).First(&setting).Error != nil {
		t.Skip("no platform login captcha scene")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	meta := ctxutil.Get(c)
	meta.App = "platformapi"
	meta.Source = ctxutil.SourcePlatform
	mobile := "13800009993"
	bootstrap.DB.Where("mobile = ?", mobile).Delete(&model.SmsLog{})
	now := util.NowUnix()
	prev := model.SmsLog{SceneID: LoginCaptcha, Mobile: mobile, Code: "1111", Content: "x", SendStatus: 1, SendTime: &now}
	if err := bootstrap.DB.Create(&prev).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("mobile = ?", mobile).Delete(&model.SmsLog{}) })
	_, _, err := Send(c, mobile, "YZMDL")
	if err == nil || err.Error() != "同一手机号1分钟只能发送1条短信" {
		t.Fatalf("rate limit: %v", err)
	}
	var n int64
	bootstrap.DB.Model(&model.SmsLog{}).Where("mobile = ? AND send_status = 2", mobile).Count(&n)
	if n < 1 {
		t.Fatal("PHP SmsMessageService writes a failed sms_log before sendLimit")
	}
}

func TestVerifyEmptySceneUsesLatest(t *testing.T) {
	if !initSMSDB(t) {
		t.Skip("no database")
	}
	mobile := fmt.Sprintf("139%08d", time.Now().UnixNano()%100000000)
	older := util.NowUnix() - 30
	newer := util.NowUnix()
	oldRow := model.SmsLog{
		SceneID: LoginCaptcha, Mobile: mobile, Code: "1111", Content: "old",
		SendStatus: 1, SendTime: &older,
	}
	newRow := model.SmsLog{
		SceneID: BindMobileCaptcha, Mobile: mobile, Code: "2222", Content: "new",
		SendStatus: 1, SendTime: &newer,
	}
	if err := bootstrap.DB.Create(&oldRow).Error; err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.DB.Create(&newRow).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("mobile = ?", mobile).Delete(&model.SmsLog{}) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	meta := ctxutil.Get(c)
	meta.App = "platformapi"
	meta.Source = ctxutil.SourcePlatform
	if Verify(c, mobile, "1111", "") {
		t.Fatal("empty scene must use latest log, not older login code")
	}
	if !Verify(c, mobile, "2222", "") {
		t.Fatal("empty scene should accept latest bind-mobile code")
	}
}

func TestSendVerifyWithoutDB(t *testing.T) {
	if bootstrap.DB != nil {
		t.Skip("DB already initialized; cache-only send is for no-DB process")
	}
	mobile := fmt.Sprintf("137%08d", time.Now().UnixNano()%100000000)
	_, code, err := Send(nil, mobile, "YZMDL")
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 4 {
		t.Fatalf("code len %d", len(code))
	}
	if !Verify(nil, mobile, code, "YZMDL") {
		t.Fatal("verify tag failed")
	}
	if Verify(nil, mobile, code, "YZMDL") {
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
	if got["url"] != "/mobile/pages/index/index" || got["page"] != "/pages/index/index" {
		t.Fatalf("php path defaults %+v", got)
	}
}

func TestNoticeBySceneMissing(t *testing.T) {
	if err := NoticeByScene(nil, 0, nil); err == nil || err.Error() != "找不到对应场景的配置" {
		t.Fatalf("%v", err)
	}
}
