package openapi

import (
	"fmt"
	"strings"
	"time"

	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/middleware"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/sms"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func LoginCodeURL(c *gin.Context) {
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信公众号配置")
		return
	}
	redirect := httpx.Str(c, "url")
	if redirect == "" {
		redirect = ctxutil.Domain(c)
	}
	response.Success(c, "获取成功", gin.H{"url": wechat.CodeURL(appID, redirect)})
}

func LoginOALogin(c *gin.Context) {
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信公众号配置")
		return
	}
	sess, err := wechat.OAuthByCode(appID, secret, httpx.Str(c, "code"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	info, err := authWechatUser(c, sess, wechat.TerminalOA, true)
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Data(c, info)
}

func LoginMnpLogin(c *gin.Context) {
	appID, secret := wechat.MnpConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信小程序配置")
		return
	}
	sess, err := wechat.Code2Session(appID, secret, httpx.Str(c, "code"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	info, err := authWechatUser(c, sess, wechat.TerminalMNP, true)
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Data(c, info)
}

func LoginGetScanCode(c *gin.Context) {
	appID, secret := wechat.OpenConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信开放平台配置")
		return
	}
	redirect := httpx.Str(c, "url")
	if redirect == "" {
		redirect = ctxutil.Domain(c) + "/pc"
	}
	state := util.MD5(fmt.Sprintf("%d%d", util.NowUnix(), ctxutil.Get(c).TenantID))
	cache.Set("web_scan_login_"+state, "1", 10*time.Minute)
	response.Data(c, gin.H{"url": wechat.ScanCodeURL(appID, redirect, state)})
}

func LoginScanLogin(c *gin.Context) {
	appID, secret := wechat.OpenConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信开放平台配置")
		return
	}
	sess, err := wechat.OAuthByCode(appID, secret, httpx.Str(c, "code"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	info, err := authWechatUser(c, sess, wechat.TerminalPC, true)
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Data(c, info)
}

func LoginMnpAuthBind(c *gin.Context) {
	bindWechatAuth(c, wechat.TerminalMNP)
}

func LoginOAAuthBind(c *gin.Context) {
	bindWechatAuth(c, wechat.TerminalOA)
}

func LoginUpdateUser(c *gin.Context) {
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&u).Updates(map[string]any{
		"nickname":    httpx.Str(c, "nickname"),
		"avatar":      filesvc.SetFileURL(c, httpx.Str(c, "avatar")),
		"is_new_user": 0,
		"update_time": now,
	})
	response.Success(c, "操作成功", nil)
}

func bindWechatAuth(c *gin.Context, terminal int) {
	uid := ctxutil.Get(c).UserID
	if uid == 0 {
		response.Fail(c, "请先登录")
		return
	}
	var sess wechat.Session
	var err error
	if terminal == wechat.TerminalMNP {
		appID, secret := wechat.MnpConfig(c)
		if appID == "" || secret == "" {
			response.Fail(c, "请先完成微信小程序配置")
			return
		}
		sess, err = wechat.Code2Session(appID, secret, httpx.Str(c, "code"))
	} else {
		appID, secret, _ := wechat.OAConfig(c)
		if appID == "" || secret == "" {
			response.Fail(c, "请先完成微信公众号配置")
			return
		}
		sess, err = wechat.OAuthByCode(appID, secret, httpx.Str(c, "code"))
	}
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	var exist model.UserAuth
	if tdb(c).Where("openid = ?", sess.Openid).First(&exist).Error == nil {
		response.Fail(c, "该微信已被绑定")
		return
	}
	if sess.Unionid != "" {
		if tdb(c).Where("unionid = ? AND user_id <> ?", sess.Unionid, uid).First(&exist).Error == nil {
			response.Fail(c, "该微信已被绑定")
			return
		}
	}
	if err := tdb(c).Create(&model.UserAuth{
		UserID: uid, Openid: sess.Openid, Unionid: sess.Unionid, Terminal: terminal, CreateTime: util.NowUnix(),
	}).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "绑定成功", nil)
}

func authWechatUser(c *gin.Context, sess wechat.Session, terminal int, create bool) (map[string]any, error) {
	if sess.Openid == "" {
		return nil, fmt.Errorf("获取openID失败")
	}
	tid := ctxutil.Get(c).TenantID
	var user model.User
	q := tdb(c).Table(model.User{}.TableName()+" u").
		Select("u.*").
		Joins("JOIN "+model.UserAuth{}.TableName()+" au ON au.user_id = u.id").
		Where("u.delete_time IS NULL AND (au.openid = ? OR (au.unionid <> '' AND au.unionid = ?))", sess.Openid, sess.Unionid)
	if tid > 0 {
		q = q.Where("u.tenant_id = ?", tid)
	}
	err := q.First(&user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	if user.ID == 0 {
		if !create {
			return map[string]any{}, nil
		}
		if err := tdb(c).Transaction(func(tx *gorm.DB) error {
			sn := util.CreateUserSN(func(v int) bool {
				var n int64
				tx.Model(&model.User{}).Where("sn = ?", v).Count(&n)
				return n > 0
			})
			now := util.NowUnix()
			avatar := config.C.Project.DefaultImage["user_avatar"]
			nickname := "用户" + util.ToString(sn)
			if terminal != wechat.TerminalMNP && sess.Nickname != "" {
				nickname = sess.Nickname
			}
			user = model.User{
				SN: sn, Account: "u" + util.ToString(sn), Nickname: nickname,
				Avatar: avatar, Channel: terminal, TenantID: tid, IsNewUser: 1, CreateTime: now,
				LoginTime: util.ZeroUnixPtr(), UpdateTime: util.ZeroUnixPtr(),
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			return tx.Create(&model.UserAuth{
				UserID: user.ID, Openid: sess.Openid, Unionid: sess.Unionid, Terminal: terminal, CreateTime: now,
			}).Error
		}); err != nil {
			return nil, err
		}
	} else {
		if user.IsDisable == 1 {
			return nil, fmt.Errorf("您的账号异常，请联系客服。")
		}
		var auth model.UserAuth
		if tdb(c).Where("user_id = ? AND openid = ?", user.ID, sess.Openid).First(&auth).Error != nil {
			tdb(c).Create(&model.UserAuth{
				UserID: user.ID, Openid: sess.Openid, Unionid: sess.Unionid, Terminal: terminal, CreateTime: util.NowUnix(),
			})
		} else if auth.Unionid == "" && sess.Unionid != "" {
			tdb(c).Model(&auth).Update("unionid", sess.Unionid)
		}
	}
	now := util.NowUnix()
	tdb(c).Model(&user).Updates(map[string]any{"login_time": now, "login_ip": ctxutil.ClientIP(c), "update_time": now})
	info := authsvc.SetUserToken(c, user.ID, terminal)
	avatar := filesvc.GetFileURL(c, firstNonEmpty(user.Avatar, config.C.Project.DefaultImage["user_avatar"]))
	return gin.H{
		"id": user.ID, "nickname": user.Nickname, "sn": user.SN, "mobile": user.Mobile,
		"avatar": avatar, "token": info["token"], "is_new_user": user.IsNewUser,
	}, nil
}

func handlePayNotify(c *gin.Context) {
	raw := middleware.ReadBody(c)
	_ = c.Request.ParseForm()
	form := c.Request.PostForm
	n := wechat.ParsePayNotify(raw, form)
	if strings.Contains(string(raw), "ciphertext") {
		n = pay.DecryptWechatV3(raw, pay.WechatCfg(c).SignKey)
	}
	if len(form) > 0 && (form.Get("trade_status") != "" || form.Get("sign") != "") {
		if !pay.AliVerifyNotify(c, form) {
			c.String(200, "fail")
			return
		}
	}
	if n.Paid && (n.Attach == "recharge" || n.Attach == "") {
		sn := wechat.RechargeSN(n.OutTradeNo)
		if sn != "" {
			var order model.RechargeOrder
			if tdb(c).Where("sn = ? AND delete_time IS NULL", sn).First(&order).Error == nil && order.PayStatus != 1 {
				_ = markRechargePaid(&order, n.TransactionID)
			}
		}
	}
	if strings.Contains(string(raw), "ciphertext") || strings.Contains(string(raw), "event_type") {
		c.JSON(200, gin.H{"code": "SUCCESS", "message": "成功"})
		return
	}
	c.String(200, "success")
}

func WechatJsConfigReal(c *gin.Context) {
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "获取jssdk失败:请先完成微信公众号配置")
		return
	}
	cfg, err := wechat.JsConfig(appID, secret, httpx.Str(c, "url"))
	if err != nil {
		response.Fail(c, "获取jssdk失败:"+err.Error())
		return
	}
	response.Data(c, cfg)
}

func UserGetMobileByMnpReal(c *gin.Context) {
	appID, secret := wechat.MnpConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信小程序配置")
		return
	}
	phone, err := wechat.PhoneNumber(appID, secret, httpx.Str(c, "code"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	u := currentUser(c)
	q := tdb(c).Where("mobile = ? AND delete_time IS NULL AND id <> ?", phone, u.ID)
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	var exist model.User
	if q.First(&exist).Error == nil {
		response.Fail(c, "手机号已被其他账号绑定")
		return
	}
	tdb(c).Model(&u).Update("mobile", phone)
	response.Success(c, "操作成功", nil)
}

func SmsSendCodeReal(c *gin.Context) {
	mobile := httpx.Str(c, "mobile")
	scene := httpx.Str(c, "scene")
	if _, _, err := sms.Send(c, mobile, scene); err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "发送成功", nil)
}
