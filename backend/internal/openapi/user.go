package openapi

import (
	"time"

	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/decorate"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pubcache"
	"likeadmin/backend/internal/ratelimit"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/workbench"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func tdb(c *gin.Context) *gorm.DB {
	return tenantdb.Use(c)
}

func userSNTaken(c *gin.Context, db *gorm.DB, v int) bool {
	if db == nil {
		return false
	}
	q := db.Model(&model.User{}).Where("sn = ? AND delete_time IS NULL", v)
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	if tid == 0 {
		q = q.Where("1 = 0")
	} else {
		q = q.Where("tenant_id = ?", tid)
	}
	var n int64
	q.Count(&n)
	return n > 0
}

func scopeTenant(db *gorm.DB, c *gin.Context) *gorm.DB {
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	if tid == 0 {
		return db.Where("1 = 0")
	}
	return db.Where("tenant_id = ?", tid)
}

func userAuthQ(c *gin.Context) *gorm.DB {
	return scopeTenant(tdb(c).Model(&model.UserAuth{}), c)
}

func articleCollectDB(c *gin.Context) *gorm.DB {
	return scopeTenant(tdb(c).Model(&model.ArticleCollect{}).Where("delete_time IS NULL"), c)
}

func userCollectsArticle(c *gin.Context, uid, articleID uint) bool {
	if uid == 0 || articleID == 0 {
		return false
	}
	articleID = visibleArticleID(c, articleID)
	if articleID == 0 {
		return false
	}
	var n int64
	articleCollectDB(c).Where("user_id = ? AND article_id = ? AND status = 1", uid, articleID).Count(&n)
	return n > 0
}

func IndexConfig(c *gin.Context) {
	tid := ctxutil.Get(c).TenantID
	ver := cfgsvc.BootVersion(tid)
	bootKey := "boot:" + util.ToString(tid) + ":" + ver + ":" + ctxutil.Scheme(c) + ":" + ctxutil.Host(c)
	var cached map[string]any
	if cache.GetJSON(bootKey, &cached) && cached != nil {
		cached["webPage"] = bootWebPage(c, cached)
		response.DataCached(c, cached, 30*time.Second)
		return
	}
	cfgsvc.Warm(c, "website", "shop_logo", "h5_favicon", "shop_name")
	cfgsvc.Warm(c, "login", "login_way", "coerce_mobile", "login_agreement", "third_auth", "wechat_auth", "qq_auth")
	cfgsvc.Warm(c, "web_page", "status", "page_status", "page_url")
	cfgsvc.Warm(c, "copyright", "config")
	websiteLogo := cfgsvc.GetString(c, "website", "shop_logo", config.C.Project.Website["shop_logo"])
	websiteIcon := cfgsvc.GetString(c, "website", "h5_favicon", config.C.Project.Website["h5_favicon"])
	payload := gin.H{
		"domain": filesvc.GetFileURL(c, ""),
		"style":  decorate.Style(c),
		"tabbar": remapTabbarArticleIDs(c, decorate.Lists(c)),
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
	}
	cache.Set(bootKey, payload, 2*time.Minute)
	response.DataCached(c, payload, 30*time.Second)
}

func bootWebPage(c *gin.Context, cached map[string]any) gin.H {
	page, _ := cached["webPage"].(map[string]any)
	if page == nil {
		page = map[string]any{}
	}
	return gin.H{
		"status":      page["status"],
		"page_status": page["page_status"],
		"page_url":    page["page_url"],
		"url":         ctxutil.Domain(c) + "/mobile",
	}
}

func IndexPolicy(c *gin.Context) {
	typ := httpx.QueryRaw(c, "type")
	response.Data(c, gin.H{
		"title":   cfgsvc.GetString(c, "agreement", typ+"_title", ""),
		"content": cfgsvc.GetString(c, "agreement", typ+"_content", ""),
	})
}

func IndexDecorate(c *gin.Context) {
	tid := ctxutil.Get(c).TenantID
	typ := httpx.QueryInt(c, "type")
	var cached any
	if pubcache.GetJSON(tid, "decorate", util.ToString(typ), &cached) {
		response.DataCached(c, cached, 30*time.Second)
		return
	}
	var p model.DecoratePage
	db := scopeTenant(tdb(c).Where("type = ?", typ), c)
	if db.First(&p).Error != nil {
		empty := []any{}
		pubcache.Set(tid, "decorate", util.ToString(typ), empty, pubcache.TTL)
		response.DataCached(c, empty, 30*time.Second)
		return
	}
	payload := gin.H{
		"type": p.Type, "name": p.Name,
		"data": applyTenantArticleIDs(c, p.Data), "meta": p.Meta,
	}
	pubcache.Set(tid, "decorate", util.ToString(typ), payload, pubcache.TTL)
	response.DataCached(c, payload, 30*time.Second)
}

func LoginRegister(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !ratelimit.Allow(c, ratelimit.KindLogin) {
		return
	}
	if !httpx.BodyPresent(c, "channel") {
		response.Fail(c, "注册来源参数缺失")
		return
	}
	account := httpx.BodyRaw(c, "account")
	if msg := util.ValidRegisterAccount(account); msg != "" {
		response.Fail(c, msg)
		return
	}
	password := httpx.BodyRaw(c, "password")
	if msg := util.ValidRegisterPassword(password); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !util.PHPRequired(httpx.Body(c), "password_confirm") {
		response.Fail(c, "请确认密码")
		return
	}
	if password != httpx.BodyRaw(c, "password_confirm") {
		response.Fail(c, "两次输入的密码不一致")
		return
	}
	tid := ctxutil.Get(c).TenantID
	if tid == 0 {
		response.Fail(c, "接口域名错误或租户不存在")
		return
	}
	var exist model.User
	if scopeTenant(tdb(c).Where("account = ? AND delete_time IS NULL", account), c).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	now := util.NowUnix()
	sn := util.CreateUserSN(func(v int) bool {
		return userSNTaken(c, tdb(c), v)
	})
	avatar := cfgsvc.GetString(c, "default_image", "user_avatar", config.C.Project.DefaultImage["user_avatar"])
	u := model.User{
		Account: account, Nickname: "用户" + util.ToString(sn),
		Password: util.CreatePassword(password, config.C.Project.UniqueIdentification),
		Channel:  httpx.BodyInt(c, "channel"), TenantID: tid, CreateTime: now,
		Avatar: avatar, SN: sn, LoginTime: util.ZeroUnixPtr(), UpdateTime: util.UnixPtr(now),
	}
	if err := tdb(c).Create(&u).Error; err != nil {
		if util.IsDuplicateKey(err) {
			response.Fail(c, "账号已存在")
			return
		}
		response.Fail(c, err.Error())
		return
	}
	workbench.OnUserCreated(tid)
	response.Result(c, response.CodeOK, 1, "注册成功", []any{})
}

func LoginAccount(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !ratelimit.Allow(c, ratelimit.KindLogin) {
		return
	}
	if !httpx.BodyPresent(c, "terminal") {
		response.Fail(c, "终端参数缺失")
		return
	}
	terminal := httpx.BodyInt(c, "terminal")
	if terminal < 1 || terminal > 6 {
		response.Fail(c, "终端参数状态值不正确")
		return
	}
	if !httpx.BodyPresent(c, "scene") {
		response.Fail(c, "场景不能为空")
		return
	}
	scene := httpx.BodyInt(c, "scene")
	if scene != 1 && scene != 2 {
		response.Fail(c, "场景值错误")
		return
	}
	if !util.LoginWayAllows(cfgsvc.Get(c, "login", "login_way", []any{"1", "2"}), scene) {
		response.Fail(c, "不支持的登录方式")
		return
	}
	account := httpx.BodyRaw(c, "account")
	if !util.PHPRequired(httpx.Body(c), "account") {
		response.Fail(c, "请输入账号")
		return
	}
	ip := ctxutil.ClientIP(c)
	if scene == 1 {
		if !cache.UserLoginSafe(ip) {
			response.Fail(c, cache.UserLoginSafeHint())
			return
		}
		// PHP LoginAccountValidate::checkConfig uses isset(), not require.
		if !util.PHPIsset(httpx.Body(c), "password") {
			response.Fail(c, "请输入密码")
			return
		}
	} else if !util.PHPIsset(httpx.Body(c), "code") {
		response.Fail(c, "请输入手机验证码")
		return
	}
	var u model.User
	if scene == 2 {
		// PHP checkCode verifies SMS before looking up the user and never checks is_disable.
		if !verifySms(c, account, httpx.BodyRaw(c, "code"), "YZMDL") {
			response.Fail(c, "验证码错误")
			return
		}
		if scopeTenant(tdb(c).Where("delete_time IS NULL AND mobile = ?", account), c).First(&u).Error != nil {
			response.Fail(c, "用户不存在")
			return
		}
	} else {
		if scopeTenant(tdb(c).Where("delete_time IS NULL AND (account = ? OR mobile = ?)", account, account), c).First(&u).Error != nil {
			response.Fail(c, "用户不存在")
			return
		}
		if u.IsDisable == 1 {
			response.Fail(c, "用户已禁用")
			return
		}
		if u.Password == "" {
			cache.RecordUserLoginFail(ip)
			response.Fail(c, "用户不存在")
			return
		}
		if u.Password != util.CreatePassword(httpx.BodyRaw(c, "password"), config.C.Project.UniqueIdentification) {
			cache.RecordUserLoginFail(ip)
			response.Fail(c, "密码错误")
			return
		}
		cache.RelieveUserLoginFail(ip)
	}
	now := util.NowUnix()
	tdb(c).Model(&u).Updates(map[string]any{"login_time": now, "login_ip": ip, "update_time": now})
	info := authsvc.SetUserToken(c, u.ID, terminal)
	response.Data(c, gin.H{
		"nickname": u.Nickname, "sn": u.SN, "mobile": u.Mobile,
		"avatar": filesvc.LoginUserAvatarURL(c, u.Avatar, config.C.Project.DefaultImage["user_avatar"]),
		"token":  info["token"],
	})
}

func LoginLogout(c *gin.Context) {
	meta := ctxutil.Get(c)
	if meta.UserInfo != nil {
		if tok := util.ToString(meta.UserInfo["token"]); tok != "" {
			authsvc.ExpireUserToken(c, tok)
		}
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
		"real_name": u.RealName, "avatar": filesvc.GetImageAttr(c, u.Avatar),
		"mobile": u.Mobile, "create_time": util.FormatDateTime(u.CreateTime),
		"is_new_user": u.IsNewUser, "user_money": util.MoneyString(u.UserMoney), "has_password": u.Password != "",
	}
	if info := ctxutil.Get(c).UserInfo; info != nil {
		term := util.ToInt(info["terminal"])
		if term == 1 || term == 2 {
			var n int64
			scopeTenant(tdb(c).Model(&model.UserAuth{}).Where("user_id = ? AND terminal = ?", u.ID, term), c).Count(&n)
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
		scopeTenant(tdb(c).Model(&model.UserAuth{}).Where("user_id = ? AND terminal IN ?", u.ID, []int{1, 2, 4}), c).Count(&n)
		hasAuth = n > 0
	}
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "sex": util.SexDesc(u.Sex), "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetImageAttr(c, u.Avatar),
		"mobile": u.Mobile, "has_auth": hasAuth, "has_password": u.Password != "",
		"create_time": util.FormatDateTime(u.CreateTime), "user_money": util.MoneyString(u.UserMoney),
		"version": config.C.Project.Version,
	})
}

func UserSetInfo(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	p := httpx.Body(c)
	if msg := util.UserSetInfoCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	field := httpx.BodyRaw(c, "field")
	value := httpx.BodyAny(c, "value")
	if field == "account" {
		var n int64
		q := scopeTenant(tdb(c).Model(&model.User{}).Where("account = ? AND id <> ? AND delete_time IS NULL", util.ToString(value), u.ID), c)
		q.Count(&n)
		if n > 0 {
			response.Fail(c, "账号已被使用!")
			return
		}
	}
	if field == "avatar" {
		value = filesvc.SetFileURL(c, util.ToString(value))
	}
	tdb(c).Model(&u).Updates(map[string]any{field: value, "update_time": util.NowUnix()})
	response.SuccessNotice(c, "操作成功")
}

func ArticleLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := scopeTenant(tdb(c).Model(&model.Article{}).Where("delete_time IS NULL AND is_show = 1"), c)
	if lists.HasParam(q, "cid") {
		db = db.Where("cid = ?", lists.ParamInt(q, "cid"))
	}
	if lists.PHPTruthy(q, "keyword") {
		db = db.Where("title LIKE ?", "%"+lists.Param(q, "keyword")+"%")
	}
	order := "sort desc, id desc"
	switch lists.Param(q, "sort") {
	case "new":
		order = "id desc"
	case "hot":
		order = "(click_actual + click_virtual) desc, id desc"
	}
	var count int64
	db.Count(&count)
	var rows []model.Article
	db.Order(order).Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	collects := map[uint]bool{}
	if uid := ctxutil.Get(c).UserID; uid > 0 && len(rows) > 0 {
		ids := make([]uint, 0, len(rows))
		for _, a := range rows {
			ids = append(ids, a.ID)
		}
		var marks []model.ArticleCollect
		articleCollectDB(c).Where("user_id = ? AND status = 1 AND article_id IN ?", uid, ids).Find(&marks)
		for _, m := range marks {
			collects[m.ArticleID] = true
		}
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc,
			"image": filesvc.GetImageAttr(c, a.Image),
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
			"collect": collects[a.ID],
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleCate(c *gin.Context) {
	tid := ctxutil.Get(c).TenantID
	var cached any
	if pubcache.GetJSON(tid, "cate", "", &cached) {
		response.Data(c, cached)
		return
	}
	var rows []model.ArticleCate
	db := scopeTenant(tdb(c).Where("delete_time IS NULL AND is_show = 1"), c)
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{"id": r.ID, "name": r.Name})
	}
	pubcache.Set(tid, "cate", "", out, pubcache.TTL)
	response.Data(c, out)
}

func SearchHot(c *gin.Context) {
	tid := ctxutil.Get(c).TenantID
	var cached any
	if pubcache.GetJSON(tid, "hot", "", &cached) {
		response.Data(c, cached)
		return
	}
	var rows []model.HotSearch
	scopeTenant(tdb(c).Model(&model.HotSearch{}), c).Order("sort desc, id desc").Find(&rows)
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, map[string]any{"name": r.Name, "sort": r.Sort})
	}
	payload := gin.H{"status": cfgsvc.GetInt(c, "hot_search", "status", 0), "data": data}
	pubcache.Set(tid, "hot", "", payload, pubcache.TTL)
	response.Data(c, payload)
}

func RechargeLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	uid := ctxutil.Get(c).UserID
	db := scopeTenant(tdb(c).Model(&model.RechargeOrder{}).Where("user_id = ? AND pay_status = 1 AND delete_time IS NULL", uid), c)
	var count int64
	db.Count(&count)
	var rows []model.RechargeOrder
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"order_amount": util.MoneyString(r.OrderAmount),
			"create_time":  util.FormatDateTime(r.CreateTime),
			"tips":         "充值" + util.ToString(util.FormatAmount(r.OrderAmount)) + "元",
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func AccountLogLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	uid := ctxutil.Get(c).UserID
	db := scopeTenant(tdb(c).Model(&model.UserAccountLog{}).Where("user_id = ? AND delete_time IS NULL", uid), c)
	if lists.Param(q, "type") == "um" {
		db = db.Where("change_type IN ?", biz.UserMoneyChangeTypes())
	}
	if lists.PHPTruthy(q, "action") {
		db = db.Where("action = ?", lists.ParamInt(q, "action"))
	}
	var count int64
	db.Count(&count)
	var rows []model.UserAccountLog
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		symbol := "+"
		if r.Action == biz.DEC {
			symbol = "-"
		}
		amt := util.MoneyString(r.ChangeAmount)
		out = append(out, map[string]any{
			"change_type":        r.ChangeType,
			"change_amount":      amt,
			"action":             r.Action,
			"create_time":        util.FormatDateTime(r.CreateTime),
			"remark":             r.Remark,
			"type_desc":          biz.ChangeTypeDesc(r.ChangeType),
			"change_amount_desc": symbol + amt,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func WechatJsConfig(c *gin.Context) { WechatJsConfigReal(c) }

func PayNotifyOK(c *gin.Context) { handlePayNotify(c) }
func AliNotify(c *gin.Context)   { handlePayNotify(c) }

func currentUser(c *gin.Context) model.User {
	var u model.User
	id := ctxutil.Get(c).UserID
	if id == 0 {
		return u
	}
	scopeTenant(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&u)
	return u
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
