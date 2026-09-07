package sms

import (
	"encoding/json"
	"fmt"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	sendTypeSMS = 2
	sendTypeOA  = 3
	sendTypeMNP = 4
)

// titleByScene mirrors PHP NoticeLogic::getTitleByScene.
func titleByScene(sendType int, oa, mnp map[string]any) string {
	switch sendType {
	case sendTypeOA:
		return util.ToString(oa["name"])
	case sendTypeMNP:
		return util.ToString(mnp["name"])
	default:
		return ""
	}
}

func addNoticeRecord(c *gin.Context, scene int, params map[string]string, tid uint) {
	if bootstrap.DB == nil {
		return
	}
	if params == nil {
		params = map[string]string{}
	}
	meta := loadNoticeMeta(c, scene)
	content := formatContent(util.ToString(meta.sms["content"]), params)
	title := titleByScene(sendTypeSMS, meta.oa, meta.mnp)
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
			TenantID: tid, UserID: userID, Title: title, Content: content,
			SceneID: scene, Read: 0, Recipient: meta.recipient, SendType: sendTypeSMS,
			NoticeType: meta.noticeType, Extra: "", CreateTime: now,
		}).Error
		return
	}
	_ = bootstrap.DB.Create(&model.NoticeRecord{
		UserID: userID, Title: title, Content: content,
		SceneID: scene, Read: 0, Recipient: meta.recipient, SendType: sendTypeSMS,
		NoticeType: meta.noticeType, Extra: "", CreateTime: now,
	}).Error
}

type noticeMeta struct {
	recipient, noticeType int
	sms, oa, mnp          map[string]any
}

func loadNoticeMeta(c *gin.Context, scene int) noticeMeta {
	found, rec, typ, sms, oa, mnp := findNoticeSetting(c, scene)
	if !found {
		return noticeMeta{recipient: 1, noticeType: 2, sms: map[string]any{}, oa: map[string]any{}, mnp: map[string]any{}}
	}
	return noticeMeta{recipient: rec, noticeType: typ, sms: sms, oa: oa, mnp: mnp}
}

func findNoticeSetting(c *gin.Context, scene int) (found bool, recipient, noticeType int, sms, oa, mnp map[string]any) {
	recipient, noticeType = 1, 2
	sms, oa, mnp = map[string]any{}, map[string]any{}, map[string]any{}
	if bootstrap.DB == nil {
		return false, recipient, noticeType, sms, oa, mnp
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
			return true, row.Recipient, row.Type, decodeNoticeJSON(row.SmsNotice), decodeNoticeJSON(row.OaNotice), decodeNoticeJSON(row.MnpNotice)
		}
		return false, recipient, noticeType, sms, oa, mnp
	}
	var row model.NoticeSetting
	if bootstrap.DB.Where("scene_id = ?", scene).First(&row).Error == nil {
		return true, row.Recipient, row.Type, decodeNoticeJSON(row.SmsNotice), decodeNoticeJSON(row.OaNotice), decodeNoticeJSON(row.MnpNotice)
	}
	return false, recipient, noticeType, sms, oa, mnp
}

// NoticeByScene mirrors PHP NoticeLogic::noticeByScene for SMS-enabled scenes.
func NoticeByScene(c *gin.Context, sceneID int, params map[string]string) error {
	if sceneID <= 0 {
		return fmt.Errorf("找不到对应场景的配置")
	}
	found, _, _, smsNotice, _, _ := findNoticeSetting(c, sceneID)
	if !found {
		return fmt.Errorf("找不到对应场景的配置")
	}
	if util.ToInt(smsNotice["status"]) != 1 {
		return fmt.Errorf("发送通知失败")
	}
	if params == nil {
		params = map[string]string{}
	}
	params = mergeNoticeParams(c, params)
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	now := util.NowUnix()
	content := formatContent(util.ToString(smsNotice["content"]), params)
	var logID uint
	if bootstrap.DB != nil {
		logID = createSMSLog(c, sceneID, params["mobile"], params["code"], content, now)
	}
	addNoticeRecord(c, sceneID, params, tid)
	if params["mobile"] != "" && params["code"] != "" {
		cache.Set(cacheKey(sceneID, params["mobile"]), params["code"], 5*time.Minute)
	}
	if err := maybeGatewaySend(c, params["mobile"], sceneID, params, logID); err != nil {
		if params["mobile"] != "" && params["code"] != "" {
			cache.Del(cacheKey(sceneID, params["mobile"]))
		}
		return err
	}
	if logID > 0 {
		updateSMSLog(c, logID, map[string]any{"send_status": 1}, "send_status = 0")
	}
	return nil
}

func mergeNoticeParams(c *gin.Context, params map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range params {
		out[k] = v
	}
	uid := uint(util.ToInt(out["user_id"]))
	if uid == 0 && c != nil {
		uid = ctxutil.Get(c).UserID
	}
	if out["url"] == "" {
		out["url"] = "/mobile/pages/index/index"
	}
	if out["page"] == "" {
		out["page"] = "/pages/index/index"
	}
	if uid == 0 || bootstrap.DB == nil {
		return out
	}
	db := tenantdb.Use(c)
	if db == nil {
		db = bootstrap.DB
	}
	q := db.Where("id = ? AND delete_time IS NULL", uid)
	if c != nil {
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		} else {
			q = q.Where("1 = 0")
		}
	}
	var u model.User
	if q.First(&u).Error != nil {
		return out
	}
	if out["nickname"] == "" {
		out["nickname"] = u.Nickname
	}
	if out["user_name"] == "" {
		out["user_name"] = u.Nickname
	}
	if out["user_sn"] == "" {
		out["user_sn"] = util.ToString(u.SN)
	}
	if out["mobile"] == "" {
		out["mobile"] = u.Mobile
	}
	return out
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
