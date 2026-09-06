package openapi

import (
	"likeadmin/backend/internal/authsvc"
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
		response.Data(c, []any{})
		return
	}
	response.Data(c, gin.H{
		"type": p.Type, "name": p.Name,
		"data": p.Data, "meta": p.Meta,
	})
}

func LoginRegister(c *gin.Context) {
	account := httpx.Str(c, "account")
	password := httpx.Str(c, "password")
	confirm := httpx.Str(c, "password_confirm")
	if account == "" || password == "" {
		response.Fail(c, "请输入账号密码")
		return
	}
	if password != confirm {
		response.Fail(c, "两次密码不一致")
		return
	}
	tid := ctxutil.Get(c).TenantID
	var exist model.User
	if tdb(c).Where("account = ? AND tenant_id = ? AND delete_time IS NULL", account, tid).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	now := util.NowUnix()
	var maxSN int
	tdb(c).Model(&model.User{}).Select("COALESCE(MAX(sn),0)").Scan(&maxSN)
	u := model.User{
		Account: account, Nickname: "用户" + util.ToString(maxSN+1),
		Password: util.CreatePassword(password, config.C.Project.UniqueIdentification),
		Channel:  httpx.Int(c, "channel"), TenantID: tid, IsNewUser: 1, CreateTime: now,
		Avatar: config.C.Project.DefaultImage["user_avatar"], SN: maxSN + 1,
	}
	if err := tdb(c).Create(&u).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "注册成功", nil)
}

func LoginAccount(c *gin.Context) {
	account := httpx.Str(c, "account")
	password := httpx.Str(c, "password")
	scene := httpx.Int(c, "scene")
	terminal := httpx.Int(c, "terminal")
	if terminal == 0 {
		terminal = 3
	}
	tid := ctxutil.Get(c).TenantID
	var u model.User
	q := tdb(c).Where("delete_time IS NULL AND (account = ? OR mobile = ?)", account, account)
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&u).Error != nil {
		response.Fail(c, "账号不存在")
		return
	}
	if u.IsDisable == 1 {
		response.Fail(c, "账号已禁用")
		return
	}
	if scene == 2 {
		if !verifySms(c, account, httpx.Str(c, "code"), "YZMDL") {
			response.Fail(c, "验证码错误")
			return
		}
	} else if u.Password != util.CreatePassword(password, config.C.Project.UniqueIdentification) {
		response.Fail(c, "密码错误")
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&u).Updates(map[string]any{"login_time": now, "login_ip": ctxutil.ClientIP(c)})
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
	hasPwd := 0
	if u.Password != "" {
		hasPwd = 1
	}
	out := gin.H{
		"id": u.ID, "sn": u.SN, "sex": u.Sex, "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"mobile": u.Mobile, "create_time": util.FormatDateTime(u.CreateTime),
		"is_new_user": u.IsNewUser, "user_money": u.UserMoney, "has_password": hasPwd,
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
	hasAuth := 0
	if tdb(c) != nil && u.ID > 0 {
		var n int64
		tdb(c).Model(&model.UserAuth{}).Where("user_id = ? AND terminal IN ?", u.ID, []int{1, 2, 4}).Count(&n)
		if n > 0 {
			hasAuth = 1
		}
	}
	hasPwd := 0
	if u.Password != "" {
		hasPwd = 1
	}
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "sex": u.Sex, "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"mobile": u.Mobile, "has_auth": hasAuth, "has_password": hasPwd,
		"create_time": util.FormatDateTime(u.CreateTime), "user_money": u.UserMoney,
		"version": config.C.Project.Version,
	})
}

func UserSetInfo(c *gin.Context) {
	u := currentUser(c)
	field := httpx.Str(c, "field")
	value := httpx.Any(c, "value")
	allow := map[string]bool{"nickname": true, "account": true, "sex": true, "avatar": true, "real_name": true}
	if !allow[field] {
		response.Fail(c, "不允许修改")
		return
	}
	if field == "avatar" {
		value = filesvc.SetFileURL(c, util.ToString(value))
	}
	tdb(c).Model(&u).Update(field, value)
	response.Success(c, "修改成功", nil)
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
