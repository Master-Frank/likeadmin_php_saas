package openapi

import (
	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/decorate"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func tdb(c *gin.Context) *gorm.DB {
	return tenantdb.Use(c)
}

func IndexConfig(c *gin.Context) {
	websiteLogo := cfgsvc.GetString(c, "website", "shop_logo", config.C.Project.Website["shop_logo"])
	websiteIcon := cfgsvc.GetString(c, "website", "h5_favicon", config.C.Project.Website["h5_favicon"])
	response.Data(c, gin.H{
		"domain": filesvc.GetFileURL(c, ""),
		"style":  decorate.Style(c),
		"tabbar": decorate.Lists(c),
		"login": gin.H{
			"login_way":       cfgsvc.Get(c, "login", "login_way", []any{"1", "2"}),
			"coerce_mobile":   cfgsvc.GetInt(c, "login", "coerce_mobile", 1),
			"login_agreement": cfgsvc.GetInt(c, "login", "login_agreement", 1),
			"third_auth":      cfgsvc.GetInt(c, "login", "third_auth", 1),
			"wechat_auth":     cfgsvc.GetInt(c, "login", "wechat_auth", 1),
			"qq_auth":         cfgsvc.GetInt(c, "login", "qq_auth", 0),
		},
		"website": gin.H{
			"h5_favicon": filesvc.GetFileURL(c, websiteIcon),
			"shop_name":  cfgsvc.GetString(c, "website", "shop_name", "likeadmin"),
			"shop_logo":  filesvc.GetFileURL(c, websiteLogo),
		},
		"webPage": gin.H{
			"status":      cfgsvc.GetInt(c, "web_page", "status", 1),
			"page_status": cfgsvc.GetInt(c, "web_page", "page_status", 0),
			"page_url":    cfgsvc.GetString(c, "web_page", "page_url", ""),
			"url":         ctxutil.Domain(c) + "/mobile",
		},
		"version":   config.C.Project.Version,
		"copyright": cfgsvc.Get(c, "copyright", "config", []any{}),
	})
}

func IndexPolicy(c *gin.Context) {
	typ := httpx.Str(c, "type")
	if typ == "service" {
		response.Data(c, gin.H{"title": cfgsvc.GetString(c, "agreement", "service_title", "服务协议"), "content": cfgsvc.GetString(c, "agreement", "service_content", "")})
		return
	}
	response.Data(c, gin.H{"title": cfgsvc.GetString(c, "agreement", "privacy_title", "隐私政策"), "content": cfgsvc.GetString(c, "agreement", "privacy_content", "")})
}

func IndexDecorate(c *gin.Context) {
	var p model.DecoratePage
	db := tdb(c).Where("type = ?", httpx.Int(c, "type"))
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&p).Error != nil {
		response.Data(c, gin.H{"type": 0, "name": "", "data": "", "meta": ""})
		return
	}
	response.Data(c, gin.H{
		"type": p.Type, "name": p.Name,
		"data": p.Data, "meta": p.Meta,
	})
}

func LoginRegister(c *gin.Context) {
	if httpx.Any(c, "channel") == nil || httpx.Int(c, "channel") == 0 {
		response.Fail(c, "注册来源参数缺失")
		return
	}
	account := httpx.Str(c, "account")
	if msg := util.ValidRegisterAccount(account); msg != "" {
		response.Fail(c, msg)
		return
	}
	password := httpx.Str(c, "password")
	if msg := util.ValidRegisterPassword(password); msg != "" {
		response.Fail(c, msg)
		return
	}
	if httpx.Str(c, "password_confirm") == "" {
		response.Fail(c, "请确认密码")
		return
	}
	if password != httpx.Str(c, "password_confirm") {
		response.Fail(c, "两次输入的密码不一致")
		return
	}
	tid := ctxutil.Get(c).TenantID
	var exist model.User
	if tdb(c).Where("account = ? AND delete_time IS NULL", account).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	now := util.NowUnix()
	sn := util.CreateUserSN(func(v int) bool {
		var n int64
		tdb(c).Model(&model.User{}).Where("sn = ?", v).Count(&n)
		return n > 0
	})
	avatar := cfgsvc.GetString(c, "default_image", "user_avatar", config.C.Project.DefaultImage["user_avatar"])
	u := model.User{
		Account: account, Nickname: "用户" + util.ToString(sn),
		Password: util.CreatePassword(password, config.C.Project.UniqueIdentification),
		Channel:  httpx.Int(c, "channel"), TenantID: tid, CreateTime: now,
		Avatar: avatar, SN: sn, LoginTime: util.ZeroUnixPtr(), UpdateTime: util.ZeroUnixPtr(),
	}
	if err := tdb(c).Create(&u).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Result(c, response.CodeOK, 1, "注册成功", []any{})
}

func LoginAccount(c *gin.Context) {
	terminal := httpx.Int(c, "terminal")
	if terminal == 0 {
		response.Fail(c, "终端参数缺失")
		return
	}
	if terminal < 1 || terminal > 6 {
		response.Fail(c, "终端参数状态值不正确")
		return
	}
	scene := httpx.Int(c, "scene")
	if scene == 0 {
		response.Fail(c, "场景不能为空")
		return
	}
	if scene != 1 && scene != 2 {
		response.Fail(c, "场景值错误")
		return
	}
	if !util.LoginWayAllows(cfgsvc.Get(c, "login", "login_way", []any{"1", "2"}), scene) {
		response.Fail(c, "不支持的登录方式")
		return
	}
	account := httpx.Str(c, "account")
	if account == "" {
		response.Fail(c, "请输入账号")
		return
	}
	ip := ctxutil.ClientIP(c)
	if scene == 1 {
		if !cache.UserLoginSafe(ip) {
			response.Fail(c, cache.UserLoginSafeHint())
			return
		}
		if httpx.Str(c, "password") == "" {
			response.Fail(c, "请输入密码")
			return
		}
	} else if httpx.Str(c, "code") == "" {
		response.Fail(c, "请输入手机验证码")
		return
	}
	tid := ctxutil.Get(c).TenantID
	var u model.User
	q := tdb(c).Where("delete_time IS NULL")
	if scene == 2 {
		q = q.Where("mobile = ?", account)
	} else {
		q = q.Where("(account = ? OR mobile = ?)", account, account)
	}
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&u).Error != nil {
		response.Fail(c, "用户不存在")
		return
	}
	if u.IsDisable == 1 {
		response.Fail(c, "用户已禁用")
		return
	}
	if scene == 2 {
		if !verifySms(c, account, httpx.Str(c, "code"), "YZMDL") {
			response.Fail(c, "验证码错误")
			return
		}
	} else {
		if u.Password == "" {
			cache.RecordUserLoginFail(ip)
			response.Fail(c, "用户不存在")
			return
		}
		if u.Password != util.CreatePassword(httpx.Str(c, "password"), config.C.Project.UniqueIdentification) {
			cache.RecordUserLoginFail(ip)
			response.Fail(c, "密码错误")
			return
		}
		cache.RelieveUserLoginFail(ip)
	}
	now := util.NowUnix()
	tdb(c).Model(&u).Updates(map[string]any{"login_time": now, "login_ip": ip})
	info := authsvc.SetUserToken(c, u.ID, terminal)
	response.Data(c, gin.H{
		"nickname": u.Nickname, "sn": u.SN, "mobile": u.Mobile,
		"avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"token":  info["token"],
	})
}

func LoginLogout(c *gin.Context) {
	meta := ctxutil.Get(c)
	if meta.UserInfo != nil {
		authsvc.ExpireUserToken(c, util.ToString(meta.UserInfo["token"]))
	}
	response.Success(c, "success", nil)
}

func UserCenter(c *gin.Context) {
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	out := gin.H{
		"id": u.ID, "sn": u.SN, "sex": util.SexDesc(u.Sex), "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"mobile": u.Mobile, "create_time": util.FormatDateTime(u.CreateTime),
		"is_new_user": u.IsNewUser, "user_money": util.MoneyString(u.UserMoney), "has_password": u.Password != "",
	}
	if info := ctxutil.Get(c).UserInfo; info != nil {
		term := util.ToInt(info["terminal"])
		if term == 1 || term == 2 {
			var n int64
			tdb(c).Model(&model.UserAuth{}).Where("user_id = ? AND terminal = ?", u.ID, term).Count(&n)
			if n > 0 {
				out["is_auth"] = 1
			} else {
				out["is_auth"] = 0
			}
		}
	}
	response.Data(c, out)
}

func UserInfo(c *gin.Context) {
	u := currentUser(c)
	hasAuth := false
	if tdb(c) != nil && u.ID > 0 {
		var n int64
		tdb(c).Model(&model.UserAuth{}).Where("user_id = ? AND terminal IN ?", u.ID, []int{1, 2, 4}).Count(&n)
		hasAuth = n > 0
	}
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "sex": util.SexDesc(u.Sex), "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"mobile": u.Mobile, "has_auth": hasAuth, "has_password": u.Password != "",
		"create_time": util.FormatDateTime(u.CreateTime), "user_money": util.MoneyString(u.UserMoney),
		"version": config.C.Project.Version,
	})
}

func UserSetInfo(c *gin.Context) {
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	field := httpx.Str(c, "field")
	if field == "" {
		response.Fail(c, "参数缺失")
		return
	}
	if _, ok := httpx.Params(c)["value"]; !ok {
		response.Fail(c, "值不存在")
		return
	}
	value := httpx.Any(c, "value")
	allow := map[string]bool{"nickname": true, "account": true, "sex": true, "avatar": true, "real_name": true}
	if !allow[field] {
		response.Fail(c, "参数错误")
		return
	}
	if field == "account" {
		var n int64
		q := tdb(c).Model(&model.User{}).Where("account = ? AND id <> ? AND delete_time IS NULL", util.ToString(value), u.ID)
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Count(&n)
		if n > 0 {
			response.Fail(c, "账号已被使用!")
			return
		}
	}
	if field == "avatar" {
		value = filesvc.SetFileURL(c, util.ToString(value))
	}
	tdb(c).Model(&u).Update(field, value)
	response.SuccessNotice(c, "操作成功")
}

func ArticleLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.Article{}).Where("delete_time IS NULL AND is_show = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if cid := lists.ParamInt(q, "cid"); cid > 0 {
		db = db.Where("cid = ?", cid)
	}
	var count int64
	db.Count(&count)
	var rows []model.Article
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	collects := map[uint]bool{}
	if uid := ctxutil.Get(c).UserID; uid > 0 && len(rows) > 0 {
		ids := make([]uint, 0, len(rows))
		for _, a := range rows {
			ids = append(ids, a.ID)
		}
		var marks []model.ArticleCollect
		tdb(c).Where("user_id = ? AND status = 1 AND article_id IN ?", uid, ids).Find(&marks)
		for _, m := range marks {
			collects[m.ArticleID] = true
		}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc,
			"image": filesvc.GetFileURL(c, a.Image),
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
			"collect": collects[a.ID],
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleCate(c *gin.Context) {
	var rows []model.ArticleCate
	db := tdb(c).Where("delete_time IS NULL AND is_show = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"id": r.ID, "name": r.Name})
	}
	response.Data(c, out)
}

func SearchHot(c *gin.Context) {
	var rows []model.HotSearch
	db := tdb(c)
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, map[string]any{"name": r.Name, "sort": r.Sort})
	}
	response.Data(c, gin.H{"status": cfgsvc.GetInt(c, "hot_search", "status", 0), "data": data})
}

func RechargeLists(c *gin.Context) {
	q := lists.Parse(c)
	uid := ctxutil.Get(c).UserID
	db := tdb(c).Model(&model.RechargeOrder{}).Where("user_id = ? AND delete_time IS NULL", uid)
	var count int64
	db.Count(&count)
	var rows []model.RechargeOrder
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func AccountLogLists(c *gin.Context) {
	q := lists.Parse(c)
	uid := ctxutil.Get(c).UserID
	db := tdb(c).Model(&model.UserAccountLog{}).Where("user_id = ? AND delete_time IS NULL", uid)
	var count int64
	db.Count(&count)
	var rows []model.UserAccountLog
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func WechatJsConfig(c *gin.Context) { WechatJsConfigReal(c) }

func PayNotifyOK(c *gin.Context) { handlePayNotify(c) }
func AliNotify(c *gin.Context)   { handlePayNotify(c) }

func LoginStub(c *gin.Context) {
	response.Fail(c, "请先完成微信开放平台配置")
}

func currentUser(c *gin.Context) model.User {
	var u model.User
	id := ctxutil.Get(c).UserID
	if id == 0 {
		return u
	}
	tdb(c).First(&u, id)
	return u
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
