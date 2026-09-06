package tenantapi

import (
	"strings"
	"time"

	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/middleware"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

const tenantLockTag = `app\common\cache\AdminAccountSafeCache`

func LoginAccount(c *gin.Context) {
	account := httpx.Str(c, "account")
	password := httpx.Str(c, "password")
	terminal := httpx.Int(c, "terminal")
	if account == "" {
		response.Fail(c, "请输入账号")
		return
	}
	if password == "" {
		response.Fail(c, "请输入密码")
		return
	}
	restrict := cfgsvc.GetInt(c, "admin_login", "login_restrictions", 1)
	times := cfgsvc.GetInt(c, "admin_login", "password_error_times", 5)
	limit := cfgsvc.GetInt(c, "admin_login", "limit_login_time", 30)
	if restrict == 1 {
		if ok, msg := authsvc.CheckLoginLock(c, tenantLockTag, times, limit); !ok {
			response.Fail(c, msg)
			return
		}
	}
	meta := ctxutil.Get(c)
	q := bootstrap.DB.Where("account = ? AND delete_time IS NULL", account)
	if meta.TenantID > 0 {
		q = q.Where("tenant_id = ?", meta.TenantID)
	}
	var admin model.TenantAdmin
	if q.First(&admin).Error != nil {
		response.Fail(c, "账号不存在")
		return
	}
	if admin.Disable == 1 {
		response.Fail(c, "账号已禁用")
		return
	}
	if admin.Password != util.CreatePassword(password, config.C.Project.UniqueIdentification) {
		if restrict == 1 {
			authsvc.RecordLoginFail(c, tenantLockTag, limit)
		}
		response.Fail(c, "密码错误")
		return
	}
	if restrict == 1 {
		authsvc.RelieveLoginFail(c, tenantLockTag)
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&admin).Updates(map[string]any{"login_time": now, "login_ip": ctxutil.ClientIP(c)})
	info := authsvc.SetTenantToken(c, admin.ID, terminal, admin.MultipointLogin)
	avatar := admin.Avatar
	if avatar == "" {
		avatar = config.C.Project.Tenant["admin_avatar"]
	}
	response.Data(c, gin.H{
		"name": info["name"], "avatar": filesvc.GetFileURL(c, avatar),
		"role_name": info["role_name"], "token": info["token"],
	})
}

func LoginLogout(c *gin.Context) {
	meta := ctxutil.Get(c)
	if meta.AdminInfo != nil {
		if token := util.ToString(meta.AdminInfo["token"]); token != "" {
			authsvc.ExpireTenantToken(token)
		}
	}
	response.Success(c, "success", nil)
}

func ConfigGet(c *gin.Context) {
	response.Data(c, gin.H{
		"oss_domain":       filesvc.GetFileURL(c, ""),
		"web_name":         cfgsvc.GetString(c, "tenant", "name", config.C.Project.Tenant["name"]),
		"web_favicon":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "tenant", "web_favicon", config.C.Project.Tenant["web_favicon"])),
		"web_logo":         filesvc.GetFileURL(c, cfgsvc.GetString(c, "tenant", "web_logo", config.C.Project.Tenant["web_logo"])),
		"login_image":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "tenant", "login_image", config.C.Project.Tenant["login_image"])),
		"copyright_config": cfgsvc.Get(c, "copyright", "config", []any{}),
	})
}

func ConfigDict(c *gin.Context) {
	typ := httpx.Str(c, "type")
	if typ == "" {
		response.Data(c, []any{})
		return
	}
	types := strings.Split(typ, ",")
	var rows []model.DictData
	bootstrap.DB.Where("type_value IN ? AND delete_time IS NULL", types).Find(&rows)
	result := map[string]any{}
	for _, t := range types {
		list := []model.DictData{}
		for _, d := range rows {
			if d.TypeValue == t {
				list = append(list, d)
			}
		}
		result[t] = list
	}
	response.Data(c, result)
}

func WorkbenchIndex(c *gin.Context) {
	now := time.Now()
	meta := ctxutil.Get(c)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	var todayNew, totalNew int64
	uq := bootstrap.DB.Model(&model.User{}).Where("delete_time IS NULL")
	if meta.TenantID > 0 {
		uq = uq.Where("tenant_id = ?", meta.TenantID)
	}
	uq.Where("create_time >= ?", todayStart).Count(&todayNew)
	uq = bootstrap.DB.Model(&model.User{}).Where("delete_time IS NULL")
	if meta.TenantID > 0 {
		uq = uq.Where("tenant_id = ?", meta.TenantID)
	}
	uq.Count(&totalNew)
	response.Data(c, gin.H{
		"version": gin.H{"version": config.C.Project.Version, "website": "www.likeadmin.cn", "name": "SaaS租户端"},
		"today": gin.H{
			"time":        now.Format("2006-01-02 15:04:05"),
			"today_sales": 0, "total_sales": 0, "today_visitor": 0, "total_visitor": 0,
			"today_new_user": todayNew, "total_new_user": totalNew, "order_num": 0, "order_sum": 0,
		},
		"menu":    []gin.H{},
		"visitor": gin.H{"date": []string{}, "list": []gin.H{{"name": "访客数", "data": []int{}}}},
		"sale":    gin.H{"date": []string{}, "list": []gin.H{{"name": "销售量", "data": []int{}}}},
		"support": []gin.H{},
	})
}

func AdminMySelf(c *gin.Context) {
	meta := ctxutil.Get(c)
	var admin model.TenantAdmin
	if bootstrap.DB.Where("id = ?", meta.AdminID).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	var menus []model.TenantSystemMenu
	q := bootstrap.DB.Where("is_disable = 0 AND type IN ?", []string{"M", "C"})
	if meta.TenantID > 0 {
		q = q.Where("tenant_id = ?", meta.TenantID)
	}
	if admin.Root != 1 {
		var roleIDs []uint
		bootstrap.DB.Model(&model.TenantAdminRole{}).Where("admin_id = ?", admin.ID).Pluck("role_id", &roleIDs)
		var menuIDs []uint
		if len(roleIDs) > 0 {
			bootstrap.DB.Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
		}
		if len(menuIDs) == 0 {
			response.Data(c, gin.H{"user": gin.H{"id": admin.ID, "account": admin.Account, "name": admin.Name, "avatar": filesvc.GetFileURL(c, admin.Avatar), "root": admin.Root}, "menu": []any{}, "permissions": []string{}})
			return
		}
		q = q.Where("id IN ?", menuIDs)
	}
	q.Order("sort desc, id asc").Find(&menus)
	maps := make([]map[string]any, 0, len(menus))
	for _, m := range menus {
		maps = append(maps, map[string]any{
			"id": m.ID, "pid": m.Pid, "type": m.Type, "name": m.Name, "icon": m.Icon, "sort": m.Sort,
			"perms": m.Perms, "paths": m.Paths, "component": m.Component, "selected": m.Selected,
			"params": m.Params, "is_cache": m.IsCache, "is_show": m.IsShow, "is_disable": m.IsDisable,
		})
	}
	perms := []string{"*"}
	if admin.Root != 1 {
		perms = []string{}
		for _, m := range menus {
			if m.Perms != "" {
				perms = append(perms, m.Perms)
			}
		}
	}
	response.Data(c, gin.H{
		"user": gin.H{"id": admin.ID, "account": admin.Account, "name": admin.Name, "avatar": filesvc.GetFileURL(c, admin.Avatar), "root": admin.Root},
		"menu": util.LinearToTree(maps, "children", "id", "pid", 0), "permissions": perms,
	})
}

func tenantDB(c *gin.Context) uint {
	return ctxutil.Get(c).TenantID
}

func UserLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.User{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if kw := lists.Param(q, "keyword"); kw != "" {
		db = db.Where("nickname LIKE ? OR account LIKE ? OR mobile LIKE ?", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
	}
	var count int64
	db.Count(&count)
	var rows []model.User
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, u := range rows {
		out = append(out, map[string]any{
			"id": u.ID, "sn": u.SN, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
			"avatar": filesvc.GetFileURL(c, u.Avatar), "sex": u.Sex, "is_disable": u.IsDisable,
			"user_money": u.UserMoney, "create_time": util.FormatDateTime(u.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func UserDetail(c *gin.Context) {
	var u model.User
	db := bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id"))
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&u).Error != nil {
		response.Fail(c, "用户不存在")
		return
	}
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
		"avatar": filesvc.GetFileURL(c, u.Avatar), "real_name": u.RealName, "sex": u.Sex,
		"is_disable": u.IsDisable, "user_money": u.UserMoney, "create_time": util.FormatDateTime(u.CreateTime),
	})
}

func UserEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	field := httpx.Str(c, "field")
	value := httpx.Any(c, "value")
	allow := map[string]bool{"account": true, "sex": true, "mobile": true, "real_name": true}
	if !allow[field] {
		response.Fail(c, "不允许修改该字段")
		return
	}
	bootstrap.DB.Model(&model.User{}).Where("id = ?", id).Update(field, value)
	response.Success(c, "修改成功", nil)
}

func ArticleLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.Article{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if title := lists.Param(q, "title"); title != "" {
		db = db.Where("title LIKE ?", "%"+title+"%")
	}
	var count int64
	db.Count(&count)
	var rows []model.Article
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "image": filesvc.GetFileURL(c, a.Image),
			"author": a.Author, "is_show": a.IsShow, "sort": a.Sort, "click_actual": a.ClickActual,
			"create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleAdd(c *gin.Context) {
	a := model.Article{
		Cid: httpx.Uint(c, "cid"), Title: httpx.Str(c, "title"), Desc: httpx.Str(c, "desc"),
		Abstract: httpx.Str(c, "abstract"), Image: filesvc.SetFileURL(c, httpx.Str(c, "image")),
		Author: httpx.Str(c, "author"), Content: httpx.Str(c, "content"),
		IsShow: httpx.Int(c, "is_show"), Sort: httpx.Int(c, "sort"),
		TenantID: tenantDB(c), CreateTime: util.NowUnix(),
	}
	bootstrap.DB.Create(&a)
	response.Success(c, "添加成功", nil)
}

func ArticleEdit(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Article{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"cid": httpx.Uint(c, "cid"), "title": httpx.Str(c, "title"), "desc": httpx.Str(c, "desc"),
		"abstract": httpx.Str(c, "abstract"), "image": filesvc.SetFileURL(c, httpx.Str(c, "image")),
		"author": httpx.Str(c, "author"), "content": httpx.Str(c, "content"),
		"is_show": httpx.Int(c, "is_show"), "sort": httpx.Int(c, "sort"), "update_time": now,
	})
	response.Success(c, "修改成功", nil)
}

func ArticleDelete(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Article{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func ArticleDetail(c *gin.Context) {
	var a model.Article
	if bootstrap.DB.First(&a, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "文章不存在")
		return
	}
	response.Data(c, a)
}

func ArticleCateLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.ArticleCate{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var count int64
	db.Count(&count)
	var rows []model.ArticleCate
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func ArticleCateAdd(c *gin.Context) {
	bootstrap.DB.Create(&model.ArticleCate{Name: httpx.Str(c, "name"), Sort: httpx.Int(c, "sort"), IsShow: httpx.Int(c, "is_show"), TenantID: tenantDB(c), CreateTime: util.NowUnix()})
	response.Success(c, "添加成功", nil)
}

func ArticleCateEdit(c *gin.Context) {
	bootstrap.DB.Model(&model.ArticleCate{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "sort": httpx.Int(c, "sort"), "is_show": httpx.Int(c, "is_show"),
	})
	response.Success(c, "修改成功", nil)
}

func ArticleCateDelete(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.ArticleCate{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func ArticleCateAll(c *gin.Context) {
	var rows []model.ArticleCate
	db := bootstrap.DB.Where("delete_time IS NULL AND is_show = 1")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc").Find(&rows)
	response.Data(c, rows)
}

func DecoratePageDetail(c *gin.Context) {
	var p model.DecoratePage
	db := bootstrap.DB.Where("type = ?", httpx.Int(c, "type"))
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&p).Error != nil {
		response.Data(c, gin.H{})
		return
	}
	response.Data(c, p)
}

func DecoratePageSave(c *gin.Context) {
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	if id > 0 {
		bootstrap.DB.Model(&model.DecoratePage{}).Where("id = ?", id).Updates(map[string]any{"data": httpx.Str(c, "data"), "update_time": now})
	} else {
		bootstrap.DB.Create(&model.DecoratePage{Type: httpx.Int(c, "type"), Data: httpx.Str(c, "data"), TenantID: tenantDB(c), CreateTime: now})
	}
	response.Success(c, "保存成功", nil)
}

func DecorateTabbarDetail(c *gin.Context) {
	var rows []model.DecorateTabbar
	db := bootstrap.DB
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Find(&rows)
	response.Data(c, gin.H{"style": cfgsvc.Get(c, "decorate", "tabbar_style", map[string]any{}), "list": rows})
}

func SettingGetWebsite(c *gin.Context) {
	response.Data(c, gin.H{
		"name":        cfgsvc.GetString(c, "tenant", "name", ""),
		"web_favicon": filesvc.GetFileURL(c, cfgsvc.GetString(c, "tenant", "web_favicon", "")),
		"web_logo":    filesvc.GetFileURL(c, cfgsvc.GetString(c, "tenant", "web_logo", "")),
		"login_image": filesvc.GetFileURL(c, cfgsvc.GetString(c, "tenant", "login_image", "")),
		"shop_name":   cfgsvc.GetString(c, "website", "shop_name", ""),
		"shop_logo":   filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "shop_logo", "")),
		"pc_logo":     filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_logo", "")),
	})
}

func SettingSetWebsite(c *gin.Context) {
	cfgsvc.Set(c, "tenant", "name", httpx.Str(c, "name"))
	cfgsvc.Set(c, "tenant", "web_favicon", filesvc.SetFileURL(c, httpx.Str(c, "web_favicon")))
	cfgsvc.Set(c, "tenant", "web_logo", filesvc.SetFileURL(c, httpx.Str(c, "web_logo")))
	cfgsvc.Set(c, "tenant", "login_image", filesvc.SetFileURL(c, httpx.Str(c, "login_image")))
	response.Success(c, "设置成功", nil)
}

func HotSearchGet(c *gin.Context) {
	var rows []model.HotSearch
	db := bootstrap.DB
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc").Find(&rows)
	response.Data(c, gin.H{"status": cfgsvc.GetInt(c, "hot_search", "status", 0), "data": rows})
}

func RechargeGetConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"status":     cfgsvc.GetInt(c, "recharge", "status", 0),
		"min_amount": cfgsvc.Get(c, "recharge", "min_amount", 0),
	})
}

func RechargeSetConfig(c *gin.Context) {
	cfgsvc.Set(c, "recharge", "status", httpx.Int(c, "status"))
	cfgsvc.Set(c, "recharge", "min_amount", httpx.Any(c, "min_amount"))
	response.Success(c, "设置成功", nil)
}

func RechargeLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.RechargeOrder{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var count int64
	db.Count(&count)
	var rows []model.RechargeOrder
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func FinanceAccountLogLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.UserAccountLog{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var count int64
	db.Count(&count)
	var rows []model.UserAccountLog
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func FinanceRefundRecord(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.RefundRecord{})
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var count int64
	db.Count(&count)
	var rows []model.RefundRecord
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func OAReplyIndex(c *gin.Context) {
	_, _, token := wechat.OAConfig(c)
	sig := c.Query("signature")
	ts := c.Query("timestamp")
	nonce := c.Query("nonce")
	if token != "" && sig != "" && !wechat.CheckOASignature(token, sig, ts, nonce) {
		c.String(401, "invalid signature")
		return
	}
	if echostr := c.Query("echostr"); echostr != "" {
		c.String(200, echostr)
		return
	}
	raw := middleware.ReadBody(c)
	msg, err := wechat.ParseOAXML(raw)
	if err != nil || msg.MsgType == "" {
		c.String(200, "success")
		return
	}
	q := bootstrap.DB.Where("delete_time IS NULL AND status = 1")
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	var rows []model.OfficialAccountReply
	q.Order("sort asc, id asc").Find(&rows)
	mapped := make([]wechat.ReplyRow, 0, len(rows))
	for _, r := range rows {
		mapped = append(mapped, wechat.ReplyRow{
			Keyword: r.Keyword, ReplyType: r.ReplyType, MatchingType: r.MatchingType,
			Content: r.Content, Status: r.Status, Sort: r.Sort,
		})
	}
	content := wechat.MatchReply(msg, mapped)
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(200, wechat.TextReplyXML(msg.FromUserName, msg.ToUserName, content))
}
