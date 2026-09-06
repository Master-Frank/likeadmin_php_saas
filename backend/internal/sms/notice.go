package sms

import (
	"encoding/json"
	"fmt"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const sendTypeSMS = 2

func addNoticeRecord(c *gin.Context, scene int, mobile, code string, tid uint) {
	if bootstrap.DB == nil {
		return
	}
	recipient, noticeType, smsNotice := loadNoticeMeta(c, scene)
	content := formatContent(util.ToString(smsNotice["content"]), map[string]string{
		"code": code, "mobile": mobile,
	})
	userID := uint(0)
	if c != nil {
		userID = ctxutil.Get(c).UserID
	}
	now := util.NowUnix()
	if tid > 0 {
		db := tenantdb.Use(c)
		if db == nil {
			db = bootstrap.DB
		}
		_ = db.Create(&model.TenantNoticeRecord{
			TenantID: tid, UserID: userID, Title: "", Content: content,
			SceneID: scene, Read: 0, Recipient: recipient, SendType: sendTypeSMS,
			NoticeType: noticeType, Extra: "", CreateTime: now,
		}).Error
		return
	}
	_ = bootstrap.DB.Create(&model.NoticeRecord{
		UserID: userID, Title: "", Content: content,
		SceneID: scene, Read: 0, Recipient: recipient, SendType: sendTypeSMS,
		NoticeType: noticeType, Extra: "", CreateTime: now,
	}).Error
}

func loadNoticeMeta(c *gin.Context, scene int) (recipient, noticeType int, sms map[string]any) {
	found, rec, typ, sms := findNoticeSetting(c, scene)
	if !found {
		return 1, 2, map[string]any{}
	}
	return rec, typ, sms
}

func findNoticeSetting(c *gin.Context, scene int) (found bool, recipient, noticeType int, sms map[string]any) {
	recipient, noticeType = 1, 2
	sms = map[string]any{}
	if bootstrap.DB == nil {
		return false, recipient, noticeType, sms
	}
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	if tid > 0 {
		var row model.TenantNoticeSetting
		db := tenantdb.Use(c)
		if db == nil {
			db = bootstrap.DB
		}
		q := db.Where("scene_id = ?", scene).Where("tenant_id = ?", tid)
		if q.First(&row).Error == nil {
			return true, row.Recipient, row.Type, decodeNoticeJSON(row.SmsNotice)
		}
	}
	var row model.NoticeSetting
	if bootstrap.DB.Where("scene_id = ?", scene).First(&row).Error == nil {
		return true, row.Recipient, row.Type, decodeNoticeJSON(row.SmsNotice)
	}
	return false, recipient, noticeType, sms
}

// NoticeByScene mirrors PHP NoticeLogic::noticeByScene for SMS-enabled scenes.
func NoticeByScene(c *gin.Context, sceneID int, params map[string]string) error {
	if sceneID <= 0 {
		return fmt.Errorf("找不到对应场景的配置")
	}
	found, _, _, smsNotice := findNoticeSetting(c, sceneID)
	if !found {
		return fmt.Errorf("找不到对应场景的配置")
	}
	if util.ToInt(smsNotice["status"]) != 1 {
		return fmt.Errorf("发送通知失败")
	}
	if params == nil {
		params = map[string]string{}
	}
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	addNoticeRecord(c, sceneID, params["mobile"], params["code"], tid)
	if err := maybeGatewaySend(c, params["mobile"], sceneID, params["code"], 0); err != nil {
		return err
	}
	return nil
}

func decodeNoticeJSON(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil || m == nil {
		return map[string]any{}
	}
	return m
}
