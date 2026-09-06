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
	if n, err := strconv.Atoi(tag); err == nil && n > 0 {
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
	if bootstrap.DB != nil && tooFrequent(c, mobile, scene) {
		return 0, "", fmt.Errorf("同一手机号1分钟只能发送1条短信")
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	code := fmt.Sprintf("%04d", n.Int64()+1000)
	now := util.NowUnix()
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	if bootstrap.DB != nil {
		row := model.TenantSmsLog{
			SceneID: scene, Mobile: mobile, Code: code, Content: "验证码" + code,
			SendStatus: 1, SendTime: &now, TenantID: tid, CreateTime: now,
		}
		_ = bootstrap.DB.Create(&row).Error
	}
	cache.Set(cacheKey(scene, mobile), code, 5*time.Minute)
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
	if got, ok := cache.Get(cacheKey(scene, mobile)); ok && got == code {
		cache.Del(cacheKey(scene, mobile))
		markLogVerified(c, mobile, code, scene)
		return true
	}
	if bootstrap.DB == nil {
		return false
	}
	now := util.NowUnix()
	q := bootstrap.DB.Model(&model.TenantSmsLog{}).
		Where("mobile = ? AND scene_id = ? AND code = ? AND is_verify = 0 AND send_status = 1 AND send_time >= ?",
			mobile, scene, code, now-5*60)
	if c != nil {
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
	}
	var row model.TenantSmsLog
	if q.Order("id desc").First(&row).Error != nil {
		return false
	}
	bootstrap.DB.Model(&row).Updates(map[string]any{"is_verify": 1, "check_num": row.CheckNum + 1})
	cache.Del(cacheKey(scene, mobile))
	return true
}

func markLogVerified(c *gin.Context, mobile, code string, scene int) {
	if bootstrap.DB == nil {
		return
	}
	q := bootstrap.DB.Model(&model.TenantSmsLog{}).
		Where("mobile = ? AND scene_id = ? AND code = ? AND is_verify = 0", mobile, scene, code)
	if c != nil {
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
	}
	q.Updates(map[string]any{"is_verify": 1})
}

func tooFrequent(c *gin.Context, mobile string, scene int) bool {
	now := util.NowUnix()
	q := bootstrap.DB.Model(&model.TenantSmsLog{}).
		Where("mobile = ? AND send_status = 1 AND scene_id = ? AND send_time >= ?", mobile, scene, now-60)
	if c != nil {
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
	}
	var n int64
	q.Count(&n)
	return n > 0
}
