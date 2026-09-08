package sms

import (
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func initNoticeDB(t *testing.T) bool {
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
	return bootstrap.DB != nil
}

func TestMergeNoticeParamsFromUser(t *testing.T) {
	if !initNoticeDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL AND nickname <> ''").First(&user).Error != nil {
		t.Skip("no tenant user")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	meta := ctxutil.Get(c)
	meta.TenantID = user.TenantID
	meta.UserID = 99990
	got := mergeNoticeParams(c, map[string]string{
		"user_id": strconv.Itoa(int(user.ID)), "code": "4321", "nickname": "stale",
	})
	if got["nickname"] != user.Nickname || got["user_name"] != user.Nickname {
		t.Fatalf("nickname %+v user=%s", got, user.Nickname)
	}
	if got["url"] != "/mobile/pages/index/index" || got["page"] != "/pages/index/index" {
		t.Fatalf("paths %+v", got)
	}
	if user.Mobile != "" && got["mobile"] != user.Mobile {
		t.Fatalf("mobile %+v want %s", got, user.Mobile)
	}
	skip := mergeNoticeParams(c, map[string]string{"user_id": "0", "code": "4321"})
	if skip["nickname"] != "" {
		t.Fatalf("user_id=0 must not use ctx UserID, got %+v", skip)
	}
}

func TestMergeNoticeParamsPlatformTenantZero(t *testing.T) {
	if !initNoticeDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL AND nickname <> ''").First(&user).Error != nil {
		t.Skip("no tenant user")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	ctxutil.Get(c).TenantID = 0
	got := mergeNoticeParams(c, map[string]string{"user_id": strconv.Itoa(int(user.ID))})
	if got["nickname"] != user.Nickname {
		t.Fatalf("tid=0 must load user by id: %+v want %s", got, user.Nickname)
	}
}

func TestNoticeBySceneRateLimit(t *testing.T) {
	if !initNoticeDB(t) {
		t.Skip("no database")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	meta := ctxutil.Get(c)
	meta.App = "platformapi"
	meta.Source = ctxutil.SourcePlatform
	mobile := "13800009992"
	bootstrap.DB.Where("mobile = ?", mobile).Delete(&model.SmsLog{})
	now := util.NowUnix()
	row := model.SmsLog{SceneID: LoginCaptcha, Mobile: mobile, Code: "1111", Content: "x", SendStatus: 1, SendTime: &now}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("mobile = ?", mobile).Delete(&model.SmsLog{}) })
	err := NoticeByScene(c, LoginCaptcha, map[string]string{"mobile": mobile, "code": "9999"})
	if err == nil || err.Error() != "同一手机号1分钟只能发送1条短信" {
		t.Fatalf("rate limit: %v", err)
	}
}

func TestNoticeBySceneUnknown(t *testing.T) {
	if !initNoticeDB(t) {
		t.Skip("no database")
	}
	if err := NoticeByScene(nil, 99999, nil); err == nil || err.Error() != "找不到对应场景的配置" {
		t.Fatalf("%v", err)
	}
}

func TestNoticeBySceneWritesSMSLog(t *testing.T) {
	if !initNoticeDB(t) {
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
	mobile := "13800009991"
	bootstrap.DB.Where("mobile = ? AND scene_id = ?", mobile, LoginCaptcha).Delete(&model.SmsLog{})
	err := NoticeByScene(c, LoginCaptcha, map[string]string{"mobile": mobile, "code": "2468"})
	var row model.SmsLog
	if bootstrap.DB.Where("mobile = ? AND scene_id = ?", mobile, LoginCaptcha).
		Order("id desc").First(&row).Error != nil {
		t.Fatalf("NoticeByScene should write sms_log before gateway: %v", err)
	}
	if row.Code != "2468" {
		t.Fatalf("code=%s", row.Code)
	}
	if err != nil && row.SendStatus != 2 {
		t.Fatalf("gateway fail send_status=%d want 2 err=%v", row.SendStatus, err)
	}
	if err == nil && row.SendStatus != 1 {
		t.Fatalf("gateway ok send_status=%d want 1", row.SendStatus)
	}
	bootstrap.DB.Delete(&row)
}

func TestAddNoticeRecordUsesParamsUserID(t *testing.T) {
	if !initNoticeDB(t) {
		t.Skip("no database")
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	meta := ctxutil.Get(c)
	meta.UserID = 99991
	meta.TenantID = 0
	addNoticeRecord(c, LoginCaptcha, map[string]string{"user_id": "7", "code": "1"}, 0)
	var row model.NoticeRecord
	if bootstrap.DB.Where("user_id = ? AND scene_id = ?", 7, LoginCaptcha).Order("id desc").First(&row).Error != nil {
		t.Fatal("notice_record should use params user_id")
	}
	if row.UserID != 7 {
		t.Fatalf("user_id=%d want 7 (not ctx %d)", row.UserID, meta.UserID)
	}
	bootstrap.DB.Delete(&row)

	addNoticeRecord(c, LoginCaptcha, map[string]string{"code": "1"}, 0)
	var row2 model.NoticeRecord
	if bootstrap.DB.Where("user_id = 0 AND scene_id = ?", LoginCaptcha).Order("id desc").First(&row2).Error != nil {
		t.Fatal("missing params user_id should write 0")
	}
	if row2.UserID != 0 {
		t.Fatalf("user_id=%d want 0", row2.UserID)
	}
	bootstrap.DB.Delete(&row2)
}
