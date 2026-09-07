package sms

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	LoginCaptcha        = 101
	BindMobileCaptcha   = 102
	ChangeMobileCaptcha = 103
	FindPasswordCaptcha = 104
)

var tagToScene = map[string]int{
	"YZMDL":  LoginCaptcha,
	"BDSJHM": BindMobileCaptcha,
	"BGSJHM": ChangeMobileCaptcha,
	"ZHDLMM": FindPasswordCaptcha,
}

var aliasToScene = map[string]int{
	"login":                           LoginCaptcha,
	"login_captcha":                   LoginCaptcha,
	"bind_mobile":                     BindMobileCaptcha,
	"bind":                            BindMobileCaptcha,
	"change_mobile":                   ChangeMobileCaptcha,
	"find_login_password":             FindPasswordCaptcha,
	"find_password":                   FindPasswordCaptcha,
	strconv.Itoa(LoginCaptcha):        LoginCaptcha,
	strconv.Itoa(BindMobileCaptcha):   BindMobileCaptcha,
	strconv.Itoa(ChangeMobileCaptcha): ChangeMobileCaptcha,
	strconv.Itoa(FindPasswordCaptcha): FindPasswordCaptcha,
}

func SceneByTag(tag string) int {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return 0
	}
	if n, ok := tagToScene[strings.ToUpper(tag)]; ok {
		return n
	}
	if n, ok := aliasToScene[strings.ToLower(tag)]; ok {
		return n
	}
	return 0
}

func cacheKey(scene int, mobile string) string {
	return fmt.Sprintf("sms_code_%d_%s", scene, mobile)
}

func Send(c *gin.Context, mobile, sceneTag string) (int, string, error) {
	scene := SceneByTag(sceneTag)
	if scene == 0 {
		return 0, "", fmt.Errorf("场景值异常")
	}
	if mobile == "" {
		return 0, "", fmt.Errorf("请输入手机号")
	}
	if bootstrap.DB != nil && tooFrequent(c, mobile) {
		return 0, "", fmt.Errorf("同一手机号1分钟只能发送1条短信")
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	code := fmt.Sprintf("%04d", n.Int64()+1000)
	now := util.NowUnix()
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	content := "验证码" + code
	if c != nil && bootstrap.DB != nil {
		found, _, _, notice := findNoticeSetting(c, scene)
		if !found {
			return 0, "", fmt.Errorf("找不到对应场景的配置")
		}
		if _, ok := notice["status"]; ok && util.ToInt(notice["status"]) != 1 {
			return 0, "", fmt.Errorf("发送通知失败")
		}
		if formatted := formatContent(util.ToString(notice["content"]), map[string]string{"code": code, "mobile": mobile}); formatted != "" {
			content = formatted
		}
	}
	var logID uint
	if bootstrap.DB != nil {
		logID = createSMSLog(c, scene, mobile, code, content, now)
		addNoticeRecord(c, scene, map[string]string{"code": code, "mobile": mobile}, tid)
	}
	cache.Set(cacheKey(scene, mobile), code, 5*time.Minute)
	if err := maybeGatewaySend(c, mobile, scene, code, logID); err != nil {
		cache.Del(cacheKey(scene, mobile))
		return 0, "", err
	}
	if logID > 0 {
		updateSMSLog(c, logID, map[string]any{"send_status": 1}, "send_status = 0")
	}
	return scene, code, nil
}

func Verify(c *gin.Context, mobile, code, sceneTag string) bool {
	if mobile == "" || code == "" {
		return false
	}
	scene := SceneByTag(sceneTag)
	if scene == 0 {
		for _, alt := range []int{LoginCaptcha, BindMobileCaptcha, ChangeMobileCaptcha, FindPasswordCaptcha} {
			if verifyScene(c, mobile, code, alt) {
				return true
			}
		}
		return false
	}
	return verifyScene(c, mobile, code, scene)
}

func verifyScene(c *gin.Context, mobile, code string, scene int) bool {
	if bootstrap.DB == nil {
		if got, ok := cache.Get(cacheKey(scene, mobile)); ok && got == code {
			cache.Del(cacheKey(scene, mobile))
			return true
		}
		return false
	}
	now := util.NowUnix()
	q := scopeSmsTenant(c, smsLogModel(c)).
		Where("mobile = ? AND scene_id = ? AND is_verify = 0 AND send_status = 1 AND send_time >= ?",
			mobile, scene, now-5*60)
	var row struct {
		ID       uint   `gorm:"column:id"`
		CheckNum int    `gorm:"column:check_num"`
		Code     string `gorm:"column:code"`
	}
	if q.Select("id, check_num, code").Order("send_time desc, id desc").First(&row).Error != nil {
		if got, ok := cache.Get(cacheKey(scene, mobile)); ok && got == code {
			cache.Del(cacheKey(scene, mobile))
			return true
		}
		return false
	}
	fields := map[string]any{"check_num": row.CheckNum + 1}
	if row.Code == code {
		fields["is_verify"] = 1
		cache.Del(cacheKey(scene, mobile))
		updateSMSLog(c, row.ID, fields, "")
		return true
	}
	updateSMSLog(c, row.ID, fields, "")
	return false
}

var captchaScenes = []int{LoginCaptcha, BindMobileCaptcha, ChangeMobileCaptcha, FindPasswordCaptcha}

func tooFrequent(c *gin.Context, mobile string) bool {
	now := util.NowUnix()
	q := scopeSmsTenant(c, smsLogModel(c)).
		Where("mobile = ? AND send_status = 1 AND scene_id IN ? AND send_time >= ?", mobile, captchaScenes, now-60)
	var n int64
	q.Count(&n)
	return n > 0
}

func platformSMS(c *gin.Context) bool {
	if c == nil {
		return false
	}
	meta := ctxutil.Get(c)
	return meta.Source == ctxutil.SourcePlatform || meta.App == "platformapi"
}

func smsLogModel(c *gin.Context) *gorm.DB {
	if platformSMS(c) {
		return bootstrap.DB.Model(&model.SmsLog{})
	}
	return bootstrap.DB.Model(&model.TenantSmsLog{})
}

func scopeSmsTenant(c *gin.Context, q *gorm.DB) *gorm.DB {
	if platformSMS(c) || c == nil {
		return q
	}
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		return q.Where("tenant_id = ?", tid)
	}
	return q.Where("1 = 0")
}

func createSMSLog(c *gin.Context, scene int, mobile, code, content string, now int64) uint {
	if platformSMS(c) {
		row := model.SmsLog{
			SceneID: scene, Mobile: mobile, Code: code, Content: content,
			SendStatus: 0, SendTime: &now, CreateTime: now,
		}
		_ = bootstrap.DB.Create(&row).Error
		return row.ID
	}
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	row := model.TenantSmsLog{
		SceneID: scene, Mobile: mobile, Code: code, Content: content,
		SendStatus: 0, SendTime: &now, TenantID: tid, CreateTime: now,
	}
	_ = bootstrap.DB.Create(&row).Error
	return row.ID
}

func updateSMSLog(c *gin.Context, logID uint, fields map[string]any, extraWhere string) {
	if logID == 0 || bootstrap.DB == nil {
		return
	}
	q := scopeSmsTenant(c, smsLogModel(c)).Where("id = ?", logID)
	if extraWhere != "" {
		q = q.Where(extraWhere)
	}
	q.Updates(fields)
}
