package sms

import (
	"net/http/httptest"
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"

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
	meta.UserID = user.ID
	got := mergeNoticeParams(c, map[string]string{"user_id": "0", "code": "4321"})
	if got["nickname"] != user.Nickname || got["user_name"] != user.Nickname {
		t.Fatalf("nickname %+v user=%s", got, user.Nickname)
	}
	if got["url"] != "/mobile/pages/index/index" || got["page"] != "/pages/index/index" {
		t.Fatalf("paths %+v", got)
	}
	if user.Mobile != "" && got["mobile"] != user.Mobile {
		t.Fatalf("mobile %+v want %s", got, user.Mobile)
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
