package openapi

import (
	"bytes"
	"fmt"
	"log"
	"strings"
	"time"

	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/middleware"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/ratelimit"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/sms"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"
	"likeadmin/backend/internal/workbench"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func requireWechatCode(c *gin.Context) bool {
	if !util.PHPRequired(httpx.Body(c), "code") {
		response.Fail(c, "code缺少")
		return false
	}
	return true
}

func LoginCodeURL(c *gin.Context) {
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先设置公众号配置")
		return
	}
	// PHP LoginController::codeUrl passes request url as-is (may be empty).
	response.Success(c, "获取成功", gin.H{"url": wechat.CodeURL(appID, httpx.QueryRaw(c, "url"))})
}

func LoginOALogin(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !requireWechatCode(c) {
		return
	}
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先设置公众号配置")
		return
	}
	sess, err := wechat.OAuthByCode(appID, secret, httpx.BodyRaw(c, "code"))
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
	if !response.RequirePOST(c) {
		return
	}
	if !requireWechatCode(c) {
		return
	}
	appID, secret := wechat.MnpConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先设置小程序配置")
		return
	}
	sess, err := wechat.Code2Session(appID, secret, httpx.BodyRaw(c, "code"))
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
	// PHP getScanCode never checks app_id/secret; empty config still returns qrconnect URL.
	appID, _ := wechat.OpenConfig(c)
	redirect := httpx.QueryRaw(c, "url")
	state := util.MD5(fmt.Sprintf("%d%d", util.NowUnix(), time.Now().UnixNano()%100000))
	cache.Set("web_scan_"+state, state, 10*time.Minute)
	response.Data(c, gin.H{"url": wechat.ScanCodeURL(appID, redirect, state)})
}

func LoginScanLogin(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.WebScanLoginCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	state := util.ToString(p["state"])
	if raw, ok := cache.Get("web_scan_" + state); !ok || raw == "" {
		response.Fail(c, "二维码已失效或不存在,请重新扫码")
		return
	}
	appID, secret := wechat.OpenConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信开放平台配置")
		return
	}
	sess, err := wechat.OAuthByCode(appID, secret, httpx.BodyRaw(c, "code"))
	if msg := scanLoginAuthErr(sess); msg != "" {
		response.Fail(c, msg)
		return
	}
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	info, err := authWechatUser(c, sess, wechat.TerminalPC, true)
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	cache.Del("web_scan_" + state)
	response.Data(c, info)
}

func LoginMnpAuthBind(c *gin.Context) {
	bindWechatAuth(c, wechat.TerminalMNP)
}

func LoginOAAuthBind(c *gin.Context) {
	bindWechatAuth(c, wechat.TerminalOA)
}

func LoginUpdateUser(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	if msg := util.LoginUpdateUserCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&u).Updates(map[string]any{
		"nickname":    httpx.BodyRaw(c, "nickname"),
		"avatar":      filesvc.SetFileURL(c, httpx.BodyRaw(c, "avatar")),
		"is_new_user": 0,
		"update_time": now,
	})
	response.SuccessNotice(c, "操作成功")
}

func bindWechatAuth(c *gin.Context, terminal int) {
	if !response.RequirePOST(c) {
		return
	}
	uid := ctxutil.Get(c).UserID
	if uid == 0 {
		response.Fail(c, "请先登录")
		return
	}
	if !requireWechatCode(c) {
		return
	}
	var sess wechat.Session
	var err error
	if terminal == wechat.TerminalMNP {
		appID, secret := wechat.MnpConfig(c)
		if appID == "" || secret == "" {
			response.Fail(c, "请先设置小程序配置")
			return
		}
		sess, err = wechat.Code2Session(appID, secret, httpx.BodyRaw(c, "code"))
	} else {
		appID, secret, _ := wechat.OAConfig(c)
		if appID == "" || secret == "" {
			response.Fail(c, "请先设置公众号配置")
			return
		}
		sess, err = wechat.OAuthByCode(appID, secret, httpx.BodyRaw(c, "code"))
	}
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	var exist model.UserAuth
	if userAuthQ(c).Where("openid = ?", sess.Openid).First(&exist).Error == nil {
		response.Fail(c, "该微信已被绑定")
		return
	}
	if sess.Unionid != "" {
		if userAuthQ(c).Where("unionid = ? AND user_id <> ?", sess.Unionid, uid).First(&exist).Error == nil {
			response.Fail(c, "该微信已被绑定")
			return
		}
	}
	now := util.NowUnix()
	if err := tdb(c).Create(&model.UserAuth{
		TenantID: ctxutil.Get(c).TenantID, UserID: uid, Openid: sess.Openid, Unionid: sess.Unionid, Terminal: terminal, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "绑定成功")
}

func authWechatUser(c *gin.Context, sess wechat.Session, terminal int, create bool) (map[string]any, error) {
	if sess.Openid == "" {
		return nil, fmt.Errorf("获取openID失败")
	}
	tid := ctxutil.Get(c).TenantID
	if tid == 0 {
		return nil, fmt.Errorf("接口域名错误或租户不存在")
	}
	var user model.User
	q := tdb(c).Table(tenantdb.Table(c, model.User{}.TableName())+" u").
		Select("u.*").
		Joins("JOIN "+tenantdb.Table(c, model.UserAuth{}.TableName())+" au ON au.user_id = u.id").
		Where("u.delete_time IS NULL AND u.tenant_id = ? AND au.tenant_id = ? AND (au.openid = ? OR (au.unionid <> '' AND au.unionid = ?))", tid, tid, sess.Openid, sess.Unionid)
	err := q.First(&user).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	now := util.NowUnix()
	ip := ctxutil.ClientIP(c)
	created := false
	if user.ID == 0 {
		if !create {
			return map[string]any{}, nil
		}
		if err := tdb(c).Transaction(func(tx *gorm.DB) error {
			sn := util.CreateUserSN(func(v int) bool {
				return userSNTaken(c, tx, v)
			})
			avatar, avErr := filesvc.FetchWechatAvatar(c, sess.Openid, sess.Headimgurl)
			if avErr != nil {
				return avErr
			}
			nickname := "用户" + util.ToString(sn)
			if terminal != wechat.TerminalMNP && sess.Nickname != "" {
				nickname = sess.Nickname
			}
			user = model.User{
				SN: sn, Account: "u" + util.ToString(sn), Nickname: nickname,
				Avatar: avatar, Channel: terminal, TenantID: tid, IsNewUser: 1, CreateTime: now,
				LoginTime: util.ZeroUnixPtr(), UpdateTime: util.UnixPtr(now),
			}
			if err := tx.Create(&user).Error; err != nil {
				return err
			}
			if err := tx.Create(&model.UserAuth{
				TenantID: tid, UserID: user.ID, Openid: sess.Openid, Unionid: sess.Unionid, Terminal: terminal, CreateTime: now, UpdateTime: util.UnixPtr(now),
			}).Error; err != nil {
				return err
			}
			return tx.Model(&user).Updates(map[string]any{"login_time": now, "login_ip": ip, "update_time": now}).Error
		}); err != nil {
			return nil, err
		}
		created = true
		workbench.OnUserCreated(tid)
	} else {
		if user.IsDisable == 1 {
			return nil, fmt.Errorf("您的账号异常，请联系客服。")
		}
		if err := tdb(c).Transaction(func(tx *gorm.DB) error {
			if user.Avatar == "" && sess.Headimgurl != "" {
				av, avErr := filesvc.FetchWechatAvatar(c, sess.Openid, sess.Headimgurl)
				if avErr != nil {
					return avErr
				}
				if av != "" {
					user.Avatar = av
					if err := tx.Model(&user).Updates(map[string]any{"avatar": av, "update_time": now}).Error; err != nil {
						return err
					}
				}
			}
			var auth model.UserAuth
			if scopeTenant(tx.Model(&model.UserAuth{}), c).Where("user_id = ? AND openid = ?", user.ID, sess.Openid).First(&auth).Error != nil {
				if err := tx.Create(&model.UserAuth{
					TenantID: tid, UserID: user.ID, Openid: sess.Openid, Unionid: sess.Unionid, Terminal: terminal, CreateTime: now, UpdateTime: util.UnixPtr(now),
				}).Error; err != nil {
					return err
				}
			} else if auth.Unionid == "" && sess.Unionid != "" {
				if err := tx.Model(&auth).Updates(map[string]any{"unionid": sess.Unionid, "update_time": now}).Error; err != nil {
					return err
				}
			}
			return tx.Model(&user).Updates(map[string]any{"login_time": now, "login_ip": ip, "update_time": now}).Error
		}); err != nil {
			return nil, err
		}
	}
	info := authsvc.SetUserToken(c, user.ID, terminal)
	return wechatUserInfo(c, user, util.ToString(info["token"]), created), nil
}

// scanLoginAuthErr mirrors PHP LoginLogic::scanLogin:
// empty($userAuth['openid']) || empty($userAuth['access_token']) → 获取用户授权信息失败
func scanLoginAuthErr(sess wechat.Session) string {
	if sess.Openid == "" || sess.AccessToken == "" {
		return "获取用户授权信息失败"
	}
	return ""
}

// wechatUserInfo matches PHP WechatUserService::getUserInfo.
// Existing users come from field('u.id,u.sn,u.mobile,u.nickname,u.avatar,u.is_disable,u.is_new_user')
// and have no account/channel. Newly created users expose those columns after createUser().
func wechatUserInfo(c *gin.Context, user model.User, token string, created bool) gin.H {
	out := gin.H{
		"id": user.ID, "sn": user.SN, "mobile": user.Mobile,
		"nickname": user.Nickname,
		"avatar":     filesvc.GetImageAttr(c, user.Avatar),
		"is_disable": user.IsDisable, "is_new_user": user.IsNewUser, "token": token,
	}
	if created {
		out["account"] = user.Account
		out["channel"] = user.Channel
	}
	return out
}

func handlePayNotify(c *gin.Context) {
	raw := middleware.ReadBody(c)
	_ = c.Request.ParseForm()
	form := c.Request.PostForm
	isV3 := strings.Contains(string(raw), "ciphertext") || strings.Contains(string(raw), "event_type")
	isXML := bytes.Contains(raw, []byte("<xml")) || bytes.Contains(raw, []byte("<XML"))

	var n wechat.PayNotify
	if isXML {
		n = wechat.ParsePayNotify(raw, nil)
		if !wechat.VerifyWechatV2XML(raw, payNotifyWechatKey(c, wechat.RechargeSN(n.OutTradeNo))) {
			c.String(200, "fail")
			return
		}
	} else if isV3 {
		if !pay.VerifyWechatV3Notify(c, raw) {
			c.JSON(200, gin.H{"code": "FAIL", "message": "验签失败"})
			return
		}
		dec, ok := pay.DecryptWechatV3WithKeys(raw, pay.CollectWechatSignKeys(ctxutil.Get(c).TenantID))
		if !ok {
			c.JSON(200, gin.H{"code": "FAIL", "message": "解密失败"})
			return
		}
		n = dec
	} else {
		n = wechat.ParsePayNotify(raw, form)
		if len(form) > 0 && (form.Get("trade_status") != "" || form.Get("sign") != "") {
			tid := ctxutil.Get(c).TenantID
			if order, err := findRechargeByNotify(n.OutTradeNo); err == nil && order != nil {
				tid = order.TenantID
			}
			if !pay.AliVerifyNotifyByTenant(tid, form) {
				c.String(200, "fail")
				return
			}
		}
	}
	if wechat.ShouldApplyRefund(n) {
		pay.ApplyRefundNotify(n)
	}
	if wechat.ShouldMarkRechargePaid(n) {
		if order, err := findRechargeByNotify(n.OutTradeNo); err == nil && order != nil && order.PayStatus != 1 {
			if err := markRechargePaid(order, n.TransactionID); err != nil {
				log.Printf("pay notify recharge %s: %v", n.OutTradeNo, err)
			}
		}
	}
	if isV3 {
		c.JSON(200, gin.H{"code": "SUCCESS", "message": "成功"})
		return
	}
	c.String(200, "success")
}

func payNotifyWechatKey(c *gin.Context, sn string) string {
	if order, err := findRechargeByNotify(sn); err == nil && order != nil {
		if cfg := pay.WechatCfgByTenant(order.TenantID); cfg.SignKey != "" {
			return cfg.SignKey
		}
	}
	return pay.WechatCfg(c).SignKey
}

func findRechargeByNotify(outTradeNo string) (*model.RechargeOrder, error) {
	if outTradeNo == "" || bootstrap.DB == nil {
		return nil, gorm.ErrRecordNotFound
	}
	seen := map[string]bool{}
	for _, sn := range []string{outTradeNo, wechat.RechargeSN(outTradeNo)} {
		if sn == "" || seen[sn] {
			continue
		}
		seen[sn] = true
		var order model.RechargeOrder
		if err := bootstrap.DB.Where("(sn = ? OR pay_sn = ?) AND delete_time IS NULL", sn, sn).First(&order).Error; err == nil {
			return &order, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func WechatJsConfigReal(c *gin.Context) {
	if msg := util.WechatJsConfigCheck(httpx.Query(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.FailSilent(c, "获取jssdk失败:请先设置公众号配置")
		return
	}
	cfg, err := wechat.JsConfig(appID, secret, httpx.QueryRaw(c, "url"))
	if err != nil {
		response.FailSilent(c, "获取jssdk失败:"+err.Error())
		return
	}
	response.Data(c, cfg)
}

func UserGetMobileByMnpReal(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !util.PHPRequired(httpx.Body(c), "code") {
		response.Fail(c, "参数缺失")
		return
	}
	appID, secret := wechat.MnpConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先设置小程序配置")
		return
	}
	phone, err := wechat.PhoneNumber(appID, secret, httpx.BodyRaw(c, "code"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	u := currentUser(c)
	q := scopeTenant(tdb(c).Where("mobile = ? AND delete_time IS NULL AND id <> ?", phone, u.ID), c)
	var exist model.User
	if q.First(&exist).Error == nil {
		response.Fail(c, "手机号已被其他账号绑定")
		return
	}
	tdb(c).Model(&u).Updates(map[string]any{"mobile": phone, "update_time": util.NowUnix()})
	response.SuccessNotice(c, "绑定成功")
}

func SmsSendCodeReal(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !ratelimit.Allow(c, ratelimit.KindSMS) {
		return
	}
	p := httpx.Body(c)
	if !util.PHPRequired(p, "mobile") {
		response.Fail(c, "请输入手机号")
		return
	}
	mobile := httpx.BodyRaw(c, "mobile")
	if util.ValidChinaMobile(mobile) != "" {
		response.Fail(c, "请输入正确手机号")
		return
	}
	if !util.PHPRequired(p, "scene") {
		response.Fail(c, "请输入场景值")
		return
	}
	scene := httpx.BodyRaw(c, "scene")
	if _, _, err := sms.Send(c, mobile, scene); err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "发送成功", nil)
}
