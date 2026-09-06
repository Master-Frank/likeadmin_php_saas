package sms

import (
	"encoding/json"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
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
		_ = bootstrap.DB.Create(&model.TenantNoticeRecord{
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
	recipient, noticeType = 1, 2
	sms = map[string]any{}
	if bootstrap.DB == nil {
		return recipient, noticeType, sms
	}
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	if tid > 0 {
		var row model.TenantNoticeSetting
		q := bootstrap.DB.Where("scene_id = ?", scene).Where("tenant_id = ?", tid)
		if q.First(&row).Error == nil {
			return row.Recipient, row.Type, decodeNoticeJSON(row.SmsNotice)
		}
	}
	var row model.NoticeSetting
	if bootstrap.DB.Where("scene_id = ?", scene).First(&row).Error == nil {
		return row.Recipient, row.Type, decodeNoticeJSON(row.SmsNotice)
	}
	return recipient, noticeType, sms
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
