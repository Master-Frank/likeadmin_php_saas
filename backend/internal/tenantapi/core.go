package tenantapi

import (
	"strings"
	"time"

	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/decorate"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/middleware"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"
	"likeadmin/backend/internal/workbench"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func tdb(c *gin.Context) *gorm.DB {
	return tenantdb.Use(c)
}

const tenantLockTag = `app\common\cache\AdminAccountSafeCache`

func LoginAccount(c *gin.Context) {
	if msg := util.LoginTerminalCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
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
	q := tdb(c).Where("account = ? AND delete_time IS NULL", account)
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
	if admin.Password == "" {
		if restrict == 1 {
			authsvc.RecordLoginFail(c, tenantLockTag, limit)
		}
		response.Fail(c, "账号不存在")
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
	tdb(c).Model(&admin).Updates(map[string]any{"login_time": now, "login_ip": ctxutil.ClientIP(c)})
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
			authsvc.ExpireTenantToken(c, token)
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
	tdb(c).Where("type_value IN ? AND delete_time IS NULL", types).Find(&rows)
	if len(rows) == 0 {
		response.Data(c, []any{})
		return
	}
	result := map[string]any{}
	for _, t := range types {
		list := make([]map[string]any, 0)
		for _, d := range rows {
			if d.TypeValue == t {
				list = append(list, dictDataMap(d))
			}
		}
		if len(list) > 0 {
			result[t] = list
		}
	}
	response.Data(c, result)
}

func dictDataMap(d model.DictData) map[string]any {
	return map[string]any{
		"id": d.ID, "name": d.Name, "value": d.Value, "type_id": d.TypeID,
		"type_value": d.TypeValue, "sort": d.Sort, "status": d.Status, "remark": d.Remark,
		"create_time": util.FormatDateTime(d.CreateTime),
		"update_time": util.FormatDateTimeOrNil(d.UpdateTime),
	}
}

func WorkbenchIndex(c *gin.Context) {
	now := time.Now()
	meta := ctxutil.Get(c)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	var todayNew, totalNew int64
	uq := tdb(c).Model(&model.User{}).Where("delete_time IS NULL")
	if meta.TenantID > 0 {
		uq = uq.Where("tenant_id = ?", meta.TenantID)
	}
	uq.Where("create_time >= ?", todayStart).Count(&todayNew)
	uq = tdb(c).Model(&model.User{}).Where("delete_time IS NULL")
	if meta.TenantID > 0 {
		uq = uq.Where("tenant_id = ?", meta.TenantID)
	}
	uq.Count(&totalNew)
	vDates, vNums := workbench.Series(now, 15, 0, 100)
	sDates, sNums := workbench.Series(now, 7, 30, 200)
	response.Data(c, gin.H{
		"version": workbench.Version(c, "tenant"),
		"today":   workbench.Today(now, todayNew, totalNew),
		"menu":    workbench.TenantMenu(c),
		"visitor": gin.H{"date": vDates, "list": []gin.H{{"name": "访客数", "data": vNums}}},
		"sale":    gin.H{"date": sDates, "list": []gin.H{{"name": "销售量", "data": sNums}}},
		"support": workbench.Support(c),
	})
}

func AdminMySelf(c *gin.Context) {
	meta := ctxutil.Get(c)
	var admin model.TenantAdmin
	if tdb(c).Where("id = ?", meta.AdminID).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	var menus []model.TenantSystemMenu
	q := tdb(c).Where("is_disable = 0 AND type IN ?", []string{"M", "C"})
	if meta.TenantID > 0 {
		q = q.Where("tenant_id = ?", meta.TenantID)
	}
	roleIDs, deptIDs, jobIDs := []uint{}, []uint{}, []uint{}
	tdb(c).Model(&model.TenantAdminRole{}).Where("admin_id = ?", admin.ID).Pluck("role_id", &roleIDs)
	tdb(c).Model(&model.TenantAdminDept{}).Where("admin_id = ?", admin.ID).Pluck("dept_id", &deptIDs)
	tdb(c).Model(&model.TenantAdminJobs{}).Where("admin_id = ?", admin.ID).Pluck("jobs_id", &jobIDs)
	if admin.Root != 1 {
		var menuIDs []uint
		if len(roleIDs) > 0 {
			tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
		}
		if len(menuIDs) == 0 {
			response.Data(c, gin.H{"user": tenantSelfUser(c, admin, roleIDs, deptIDs, jobIDs), "menu": []any{}, "permissions": []string{}})
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
			"tenant_id": m.TenantID, "create_time": util.FormatDateTime(m.CreateTime),
			"update_time": util.FormatDateTimeOrNil(m.UpdateTime),
		})
	}
	response.Data(c, gin.H{
		"user":        tenantSelfUser(c, admin, roleIDs, deptIDs, jobIDs),
		"menu":        util.LinearToTree(maps, "children", "id", "pid", 0),
		"permissions": tenantBtnAuth(c, admin, roleIDs),
	})
}

func tenantBtnAuth(c *gin.Context, admin model.TenantAdmin, roleIDs []uint) []string {
	if admin.Root == 1 {
		return []string{"*"}
	}
	var menuIDs []uint
	if len(roleIDs) > 0 {
		tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
	}
	return authsvc.BtnAuth(false, tenantMenuPerms(c, menuIDs, true), tenantMenuPerms(c, nil, false))
}

func tenantMenuPerms(c *gin.Context, menuIDs []uint, filterIDs bool) []string {
	q := tdb(c).Model(&model.TenantSystemMenu{}).Where("is_disable = 0 AND perms <> ''")
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if filterIDs {
		if len(menuIDs) == 0 {
			return []string{}
		}
		q = q.Where("id IN ?", menuIDs)
	}
	var perms []string
	q.Distinct("perms").Pluck("perms", &perms)
	return perms
}

func tenantSelfUser(c *gin.Context, admin model.TenantAdmin, roleIDs, deptIDs, jobIDs []uint) gin.H {
	if roleIDs == nil {
		roleIDs = []uint{}
	}
	if deptIDs == nil {
		deptIDs = []uint{}
	}
	if jobIDs == nil {
		jobIDs = []uint{}
	}
	return gin.H{
		"id": admin.ID, "account": admin.Account, "name": admin.Name,
		"avatar": filesvc.GetFileURL(c, firstNonEmpty(admin.Avatar, config.C.Project.Tenant["admin_avatar"])),
		"root":   admin.Root, "disable": admin.Disable, "multipoint_login": admin.MultipointLogin,
		"role_id": roleIDs, "dept_id": deptIDs, "jobs_id": jobIDs,
	}
}

func tenantDB(c *gin.Context) uint {
	return ctxutil.Get(c).TenantID
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func UserLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.User{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if kw := lists.Param(q, "keyword"); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("sn LIKE ? OR nickname LIKE ? OR account LIKE ? OR mobile LIKE ?", like, like, like, like)
	}
	if ch := lists.Param(q, "channel"); ch != "" {
		db = db.Where("channel = ?", lists.ParamInt(q, "channel"))
	}
	if start := lists.Param(q, "create_time_start"); start != "" {
		if ts := util.ParseDateTime(start); ts > 0 {
			db = db.Where("create_time >= ?", ts)
		}
	}
	if end := lists.Param(q, "create_time_end"); end != "" {
		if ts := util.ParseDateTime(end); ts > 0 {
			db = db.Where("create_time <= ?", ts)
		}
	}
	var count int64
	db.Count(&count)
	var rows []model.User
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, u := range rows {
		out = append(out, map[string]any{
			"id": u.ID, "sn": u.SN, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
			"avatar": filesvc.GetFileURL(c, u.Avatar), "sex": util.SexDesc(u.Sex),
			"channel": util.ChannelDesc(u.Channel), "is_disable": u.IsDisable,
			"login_time":  util.FormatDateTimePtr(u.LoginTime),
			"create_time": util.FormatDateTime(u.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func UserDetail(c *gin.Context) {
	var u model.User
	db := tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id"))
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&u).Error != nil {
		response.Fail(c, "用户不存在")
		return
	}
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
		"avatar": filesvc.GetFileURL(c, u.Avatar), "real_name": u.RealName,
		"sex": util.SexDesc(u.Sex), "sexCode": u.Sex, "channel": util.ChannelDesc(u.Channel),
		"is_disable": u.IsDisable, "user_money": util.MoneyString(u.UserMoney),
		"login_time": util.FormatDateTimePtr(u.LoginTime), "create_time": util.FormatDateTime(u.CreateTime),
	})
}

func UserEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	field := httpx.Str(c, "field")
	value := httpx.Any(c, "value")
	if id == 0 {
		response.Fail(c, "请选择用户")
		return
	}
	if field == "" {
		response.Fail(c, "请选择操作")
		return
	}
	if strings.TrimSpace(util.ToString(value)) == "" {
		response.Fail(c, "请输入内容")
		return
	}
	allow := map[string]bool{"account": true, "sex": true, "mobile": true, "real_name": true}
	if !allow[field] {
		response.Fail(c, "用户信息不允许更新")
		return
	}
	var user model.User
	db := tdb(c).Where("id = ? AND delete_time IS NULL", id)
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&user).Error != nil {
		response.Fail(c, "用户不存在！")
		return
	}
	switch field {
	case "account":
		var exist model.User
		q := tdb(c).Where("id <> ? AND account = ? AND delete_time IS NULL", id, util.ToString(value))
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		if q.First(&exist).Error == nil {
			response.Fail(c, "账号已被使用")
			return
		}
	case "mobile":
		mobile := util.ToString(value)
		if msg := util.ValidChinaMobile(mobile); msg != "" && msg != "请输入内容" {
			response.Fail(c, msg)
			return
		}
		var exist model.User
		q := tdb(c).Where("id <> ? AND mobile = ? AND delete_time IS NULL", id, mobile)
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		if q.First(&exist).Error == nil {
			response.Fail(c, "手机号码已存在")
			return
		}
	}
	tdb(c).Model(&model.User{}).Where("id = ?", id).Update(field, value)
	response.SuccessNotice(c, "操作成功")
}

func ArticleLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.Article{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if title := lists.Param(q, "title"); title != "" {
		db = db.Where("title LIKE ?", "%"+title+"%")
	}
	if cid := lists.ParamInt(q, "cid"); cid > 0 {
		db = db.Where("cid = ?", cid)
	}
	if lists.Param(q, "is_show") != "" {
		db = db.Where("is_show = ?", lists.ParamInt(q, "is_show"))
	}
	var count int64
	db.Count(&count)
	var rows []model.Article
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	cates := map[uint]string{}
	var cateRows []model.ArticleCate
	tdb(c).Find(&cateRows)
	for _, cate := range cateRows {
		cates[cate.ID] = cate.Name
	}
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
			"image": filesvc.GetFileURL(c, a.Image), "author": a.Author, "content": a.Content,
			"is_show": a.IsShow, "sort": a.Sort, "click_virtual": a.ClickVirtual, "click_actual": a.ClickActual,
			"click": a.ClickActual + a.ClickVirtual, "cate_name": cates[a.Cid],
			"tenant_id": a.TenantID, "create_time": util.FormatDateTime(a.CreateTime),
			"update_time": util.FormatDateTimeOrNil(a.UpdateTime),
			"delete_time": util.FormatDateTimeOrNil(a.DeleteTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleAdd(c *gin.Context) {
	if msg := articleWriteCheck(c, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	a := model.Article{
		Cid: httpx.Uint(c, "cid"), Title: httpx.Str(c, "title"), Desc: httpx.Str(c, "desc"),
		Abstract: httpx.Str(c, "abstract"), Image: filesvc.SetFileURL(c, httpx.Str(c, "image")),
		Author: httpx.Str(c, "author"), Content: httpx.Str(c, "content"),
		IsShow: httpx.Int(c, "is_show"), Sort: httpx.Int(c, "sort"),
		ClickVirtual: httpx.Int(c, "click_virtual"),
		TenantID:     tenantDB(c), CreateTime: util.NowUnix(),
	}
	tdb(c).Create(&a)
	response.SuccessNotice(c, "添加成功")
}

func articleWriteCheck(c *gin.Context, needID bool) string {
	if needID && httpx.Uint(c, "id") == 0 {
		return "资讯id不能为空"
	}
	if needID {
		var a model.Article
		if tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&a).Error != nil {
			return "资讯不存在"
		}
	}
	if httpx.Str(c, "title") == "" {
		return "标题不能为空"
	}
	if len(httpx.Str(c, "title")) > 255 {
		return "标题长度须在1-255位字符"
	}
	if httpx.Uint(c, "cid") == 0 {
		return "所属栏目必须存在"
	}
	if raw := httpx.Any(c, "is_show"); raw == nil || util.ToString(raw) == "" {
		return "是否显示必须存在"
	}
	if show := httpx.Int(c, "is_show"); show != 0 && show != 1 {
		return "是否显示取值异常"
	}
	return ""
}

func ArticleEdit(c *gin.Context) {
	if msg := articleWriteCheck(c, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&model.Article{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"cid": httpx.Uint(c, "cid"), "title": httpx.Str(c, "title"), "desc": httpx.Str(c, "desc"),
		"abstract": httpx.Str(c, "abstract"), "image": filesvc.SetFileURL(c, httpx.Str(c, "image")),
		"author": httpx.Str(c, "author"), "content": httpx.Str(c, "content"),
		"is_show": httpx.Int(c, "is_show"), "sort": httpx.Int(c, "sort"),
		"click_virtual": httpx.Int(c, "click_virtual"), "update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func ArticleDelete(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "资讯id不能为空")
		return
	}
	var a model.Article
	if tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&a).Error != nil {
		response.Fail(c, "资讯不存在")
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&model.Article{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.SuccessNotice(c, "删除成功")
}

func ArticleDetail(c *gin.Context) {
	var a model.Article
	if tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&a).Error != nil {
		response.Fail(c, "资讯不存在")
		return
	}
	response.Data(c, map[string]any{
		"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
		"image": filesvc.GetFileURL(c, a.Image), "author": a.Author,
		"content": filesvc.RewriteContentDomains(c, a.Content),
		"is_show": a.IsShow, "sort": a.Sort, "click_virtual": a.ClickVirtual, "click_actual": a.ClickActual,
		"click": a.ClickActual + a.ClickVirtual, "tenant_id": a.TenantID,
		"create_time": util.FormatDateTime(a.CreateTime),
		"update_time": util.FormatDateTimeOrNil(a.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(a.DeleteTime),
	})
}

func ArticleCateLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.ArticleCate{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	var count int64
	db.Count(&count)
	var rows []model.ArticleCate
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, articleCateMap(c, r))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func articleCateWriteCheck(c *gin.Context, needID bool) string {
	if needID {
		if httpx.Uint(c, "id") == 0 {
			return "资讯分类id不能为空"
		}
		var row model.ArticleCate
		if tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&row).Error != nil {
			return "资讯分类不存在"
		}
	}
	if name := httpx.Str(c, "name"); name == "" {
		return "资讯分类不能为空"
	} else if n := len([]rune(name)); n < 1 || n > 90 {
		return "资讯分类长度须在1-90位字符"
	}
	if msg := util.ArticleCateShowCheck(httpx.Params(c)); msg != "" {
		return msg
	}
	if sort := httpx.Int(c, "sort"); sort < 0 {
		return "排序值不正确"
	}
	return ""
}

func ArticleCateAdd(c *gin.Context) {
	if msg := articleCateWriteCheck(c, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Create(&model.ArticleCate{Name: httpx.Str(c, "name"), Sort: httpx.Int(c, "sort"), IsShow: httpx.Int(c, "is_show"), TenantID: tenantDB(c), CreateTime: util.NowUnix()})
	response.SuccessNotice(c, "添加成功")
}

func ArticleCateEdit(c *gin.Context) {
	if msg := articleCateWriteCheck(c, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Model(&model.ArticleCate{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "sort": httpx.Int(c, "sort"), "is_show": httpx.Int(c, "is_show"),
	})
	response.SuccessNotice(c, "编辑成功")
}

func ArticleCateDelete(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "资讯分类id不能为空")
		return
	}
	var row model.ArticleCate
	if tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&row).Error != nil {
		response.Fail(c, "资讯分类不存在")
		return
	}
	var n int64
	tdb(c).Model(&model.Article{}).Where("cid = ? AND delete_time IS NULL", httpx.Uint(c, "id")).Count(&n)
	if n > 0 {
		response.Fail(c, "资讯分类已使用，请先删除绑定该资讯分类的资讯")
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&model.ArticleCate{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.SuccessNotice(c, "删除成功")
}

func ArticleCateAll(c *gin.Context) {
	var rows []model.ArticleCate
	db := tdb(c).Where("delete_time IS NULL AND is_show = 1")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, articleCateMap(c, r))
	}
	response.Data(c, out)
}

func articleCateMap(c *gin.Context, r model.ArticleCate) map[string]any {
	var n int64
	tdb(c).Model(&model.Article{}).Where("cid = ? AND delete_time IS NULL", r.ID).Count(&n)
	showDesc := "停用"
	if r.IsShow == 1 {
		showDesc = "启用"
	}
	return map[string]any{
		"id": r.ID, "name": r.Name, "sort": r.Sort, "is_show": r.IsShow,
		"is_show_desc": showDesc, "article_count": n, "tenant_id": r.TenantID,
		"create_time": util.FormatDateTime(r.CreateTime),
		"update_time": util.FormatDateTimeOrNil(r.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(r.DeleteTime),
	}
}

func decoratePayload(c *gin.Context, key string) string {
	v := httpx.Any(c, key)
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return util.EncodeJSON(v)
}

func DecoratePageDetail(c *gin.Context) {
	var p model.DecoratePage
	db := tdb(c).Where("type = ?", httpx.Int(c, "type"))
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&p).Error != nil {
		response.Success(c, "获取成功", gin.H{})
		return
	}
	response.Success(c, "获取成功", gin.H{
		"id": p.ID, "type": p.Type, "name": p.Name,
		"data": p.Data, "meta": p.Meta, "tenant_id": p.TenantID,
		"create_time": util.FormatDateTime(p.CreateTime),
		"update_time": util.FormatDateTimeOrNil(p.UpdateTime),
	})
}

func DecoratePageSave(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	if httpx.Str(c, "type") == "" && httpx.Int(c, "type") == 0 && httpx.Any(c, "type") == nil {
		response.Fail(c, "装修类型参数缺失")
		return
	}
	data := decoratePayload(c, "data")
	if data == "" {
		response.Fail(c, "装修信息参数缺失")
		return
	}
	var page model.DecoratePage
	if tdb(c).Where("id = ?", id).First(&page).Error != nil {
		response.Fail(c, "信息不存在")
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&page).Updates(map[string]any{
		"type": httpx.Int(c, "type"), "data": data,
		"meta": decoratePayload(c, "meta"), "update_time": now,
	})
	response.SuccessNotice(c, "操作成功")
}

func DecorateTabbarDetail(c *gin.Context) {
	response.Data(c, gin.H{"style": decorate.Style(c), "list": decorate.Lists(c)})
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
		"pc_title":    cfgsvc.GetString(c, "website", "pc_title", ""),
		"pc_ico":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_ico", "")),
		"pc_desc":     cfgsvc.GetString(c, "website", "pc_desc", ""),
		"pc_keywords": cfgsvc.GetString(c, "website", "pc_keywords", ""),
		"h5_favicon":  filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "h5_favicon", "")),
	})
}

func SettingSetWebsite(c *gin.Context) {
	if msg := util.TenantWebSettingCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "tenant", "name", httpx.Str(c, "name"))
	cfgsvc.Set(c, "tenant", "web_favicon", filesvc.SetFileURL(c, httpx.Str(c, "web_favicon")))
	cfgsvc.Set(c, "tenant", "web_logo", filesvc.SetFileURL(c, httpx.Str(c, "web_logo")))
	cfgsvc.Set(c, "tenant", "login_image", filesvc.SetFileURL(c, httpx.Str(c, "login_image")))
	cfgsvc.Set(c, "website", "shop_name", httpx.Str(c, "shop_name"))
	cfgsvc.Set(c, "website", "shop_logo", filesvc.SetFileURL(c, httpx.Str(c, "shop_logo")))
	cfgsvc.Set(c, "website", "pc_logo", filesvc.SetFileURL(c, httpx.Str(c, "pc_logo")))
	cfgsvc.Set(c, "website", "pc_title", httpx.Str(c, "pc_title"))
	cfgsvc.Set(c, "website", "pc_ico", filesvc.SetFileURL(c, httpx.Str(c, "pc_ico")))
	cfgsvc.Set(c, "website", "pc_desc", httpx.Str(c, "pc_desc"))
	cfgsvc.Set(c, "website", "pc_keywords", httpx.Str(c, "pc_keywords"))
	cfgsvc.Set(c, "website", "h5_favicon", filesvc.SetFileURL(c, httpx.Str(c, "h5_favicon")))
	response.SuccessNotice(c, "设置成功")
}

func HotSearchGet(c *gin.Context) {
	var rows []model.HotSearch
	db := tdb(c)
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	data := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		data = append(data, map[string]any{"name": r.Name, "sort": r.Sort})
	}
	response.Data(c, gin.H{"status": cfgsvc.GetInt(c, "hot_search", "status", 0), "data": data})
}

func RechargeGetConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"status":     cfgsvc.GetInt(c, "recharge", "status", 0),
		"min_amount": cfgsvc.Get(c, "recharge", "min_amount", 0),
	})
}

func RechargeSetConfig(c *gin.Context) {
	p := httpx.Params(c)
	if _, ok := p["status"]; ok {
		cfgsvc.Set(c, "recharge", "status", httpx.Int(c, "status"))
	}
	if _, ok := p["min_amount"]; ok {
		cfgsvc.Set(c, "recharge", "min_amount", httpx.Any(c, "min_amount"))
	}
	response.SuccessNotice(c, "操作成功")
}

func RechargeLists(c *gin.Context) {
	q := lists.Parse(c)
	ro := tenantdb.Table(c, model.RechargeOrder{}.TableName())
	u := tenantdb.Table(c, model.User{}.TableName())
	db := tdb(c).Table(ro + " AS ro").Joins("JOIN " + u + " AS u ON u.id = ro.user_id").Where("ro.delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("ro.tenant_id = ?", tid)
	}
	if sn := lists.Param(q, "sn"); sn != "" {
		db = db.Where("ro.sn = ?", sn)
	}
	if lists.Param(q, "pay_way") != "" {
		db = db.Where("ro.pay_way = ?", lists.ParamInt(q, "pay_way"))
	}
	if lists.Param(q, "pay_status") != "" {
		db = db.Where("ro.pay_status = ?", lists.ParamInt(q, "pay_status"))
	}
	if info := lists.Param(q, "user_info"); info != "" {
		like := "%" + info + "%"
		db = db.Where("u.sn LIKE ? OR u.nickname LIKE ? OR u.mobile LIKE ? OR u.account LIKE ?", like, like, like, like)
	}
	if q.StartTime != "" && q.EndTime != "" {
		db = db.Where("ro.create_time BETWEEN ? AND ?", util.ParseDateTime(q.StartTime), util.ParseDateTime(q.EndTime))
	}
	var count int64
	db.Count(&count)
	type row struct {
		ID           uint    `gorm:"column:id"`
		SN           string  `gorm:"column:sn"`
		OrderAmount  float64 `gorm:"column:order_amount"`
		PayWay       int     `gorm:"column:pay_way"`
		PayTime      *int64  `gorm:"column:pay_time"`
		PayStatus    int     `gorm:"column:pay_status"`
		CreateTime   int64   `gorm:"column:create_time"`
		RefundStatus int     `gorm:"column:refund_status"`
		Avatar       string  `gorm:"column:avatar"`
		Nickname     string  `gorm:"column:nickname"`
		Account      string  `gorm:"column:account"`
	}
	var rows []row
	db.Select("ro.id,ro.sn,ro.order_amount,ro.pay_way,ro.pay_time,ro.pay_status,ro.create_time,ro.refund_status,u.avatar,u.nickname,u.account").
		Order("ro.id desc").Offset(q.Offset).Limit(q.PageSize).Scan(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		payTime := ""
		if r.PayTime != nil && *r.PayTime > 0 {
			payTime = util.FormatDateTime(*r.PayTime)
		}
		out = append(out, map[string]any{
			"id": r.ID, "sn": r.SN, "order_amount": r.OrderAmount, "pay_way": r.PayWay,
			"pay_time": payTime, "pay_status": r.PayStatus, "refund_status": r.RefundStatus,
			"create_time": util.FormatDateTime(r.CreateTime),
			"avatar":      filesvc.GetFileURL(c, r.Avatar), "nickname": r.Nickname, "account": r.Account,
			"pay_status_text": util.PayStatusText(r.PayStatus), "pay_way_text": util.PayWayText(r.PayWay),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func FinanceAccountLogLists(c *gin.Context) {
	q := lists.Parse(c)
	al := tenantdb.Table(c, model.UserAccountLog{}.TableName())
	u := tenantdb.Table(c, model.User{}.TableName())
	db := tdb(c).Table(al + " AS al").Joins("JOIN " + u + " AS u ON u.id = al.user_id")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("al.tenant_id = ?", tid)
	}
	if lists.Param(q, "change_type") != "" {
		db = db.Where("al.change_type = ?", lists.ParamInt(q, "change_type"))
	}
	if lists.Param(q, "type") == "um" {
		db = db.Where("al.change_type IN ?", []int{biz.UMDecAdmin, biz.UMDecRechargeRefund, biz.UMIncAdmin, biz.UMIncRecharge})
	}
	if info := lists.Param(q, "user_info"); info != "" {
		like := "%" + info + "%"
		db = db.Where("u.sn LIKE ? OR u.nickname LIKE ? OR u.mobile LIKE ? OR u.account LIKE ?", like, like, like, like)
	}
	if q.StartTime != "" {
		db = db.Where("al.create_time >= ?", util.ParseDateTime(q.StartTime))
	}
	if q.EndTime != "" {
		db = db.Where("al.create_time <= ?", util.ParseDateTime(q.EndTime))
	}
	var count int64
	db.Count(&count)
	type row struct {
		Nickname     string  `gorm:"column:nickname"`
		Account      string  `gorm:"column:account"`
		SN           string  `gorm:"column:sn"`
		Avatar       string  `gorm:"column:avatar"`
		Mobile       string  `gorm:"column:mobile"`
		Action       int     `gorm:"column:action"`
		ChangeAmount float64 `gorm:"column:change_amount"`
		LeftAmount   float64 `gorm:"column:left_amount"`
		ChangeType   int     `gorm:"column:change_type"`
		SourceSN     string  `gorm:"column:source_sn"`
		CreateTime   int64   `gorm:"column:create_time"`
	}
	var rows []row
	db.Select("u.nickname,u.account,u.sn,u.avatar,u.mobile,al.action,al.change_amount,al.left_amount,al.change_type,al.source_sn,al.create_time").
		Order("al.id desc").Offset(q.Offset).Limit(q.PageSize).Scan(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		sym := "-"
		if r.Action == biz.INC {
			sym = "+"
		}
		out = append(out, map[string]any{
			"nickname": r.Nickname, "account": r.Account, "sn": r.SN,
			"avatar": filesvc.GetFileURL(c, r.Avatar), "mobile": r.Mobile,
			"action": r.Action, "change_amount": sym + util.ToString(r.ChangeAmount),
			"left_amount": r.LeftAmount, "change_type": r.ChangeType, "source_sn": r.SourceSN,
			"create_time":      util.FormatDateTime(r.CreateTime),
			"change_type_desc": biz.UMChangeTypeDesc[util.ToString(r.ChangeType)],
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func FinanceRefundRecord(c *gin.Context) {
	q := lists.Parse(c)
	rt := tenantdb.Table(c, model.RefundRecord{}.TableName())
	u := tenantdb.Table(c, model.User{}.TableName())
	base := tdb(c).Table(rt + " AS r").Joins("JOIN " + u + " AS u ON u.id = r.user_id")
	if tid := tenantDB(c); tid > 0 {
		base = base.Where("r.tenant_id = ?", tid)
	}
	if sn := lists.Param(q, "sn"); sn != "" {
		base = base.Where("r.sn = ?", sn)
	}
	if osn := lists.Param(q, "order_sn"); osn != "" {
		base = base.Where("r.order_sn = ?", osn)
	}
	if lists.Param(q, "refund_type") != "" {
		base = base.Where("r.refund_type = ?", lists.ParamInt(q, "refund_type"))
	}
	if info := lists.Param(q, "user_info"); info != "" {
		like := "%" + info + "%"
		base = base.Where("u.sn LIKE ? OR u.nickname LIKE ? OR u.mobile LIKE ? OR u.account LIKE ?", like, like, like, like)
	}
	if q.StartTime != "" {
		base = base.Where("r.create_time >= ?", util.ParseDateTime(q.StartTime))
	}
	if q.EndTime != "" {
		base = base.Where("r.create_time <= ?", util.ParseDateTime(q.EndTime))
	}
	extendWhere := func(db *gorm.DB) *gorm.DB {
		db = db.Table(rt + " AS r").Joins("JOIN " + u + " AS u ON u.id = r.user_id")
		if tid := tenantDB(c); tid > 0 {
			db = db.Where("r.tenant_id = ?", tid)
		}
		if sn := lists.Param(q, "sn"); sn != "" {
			db = db.Where("r.sn = ?", sn)
		}
		if osn := lists.Param(q, "order_sn"); osn != "" {
			db = db.Where("r.order_sn = ?", osn)
		}
		if lists.Param(q, "refund_type") != "" {
			db = db.Where("r.refund_type = ?", lists.ParamInt(q, "refund_type"))
		}
		if info := lists.Param(q, "user_info"); info != "" {
			like := "%" + info + "%"
			db = db.Where("u.sn LIKE ? OR u.nickname LIKE ? OR u.mobile LIKE ? OR u.account LIKE ?", like, like, like, like)
		}
		if q.StartTime != "" {
			db = db.Where("r.create_time >= ?", util.ParseDateTime(q.StartTime))
		}
		if q.EndTime != "" {
			db = db.Where("r.create_time <= ?", util.ParseDateTime(q.EndTime))
		}
		return db
	}
	if lists.Param(q, "refund_status") != "" {
		base = base.Where("r.refund_status = ?", lists.ParamInt(q, "refund_status"))
	}
	var count int64
	base.Count(&count)
	type row struct {
		model.RefundRecord
		Nickname string `gorm:"column:nickname"`
		Avatar   string `gorm:"column:avatar"`
	}
	var rows []row
	base.Select("r.*,u.nickname,u.avatar").Order("r.id desc").Offset(q.Offset).Limit(q.PageSize).Scan(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "sn": r.SN, "user_id": r.UserID, "order_id": r.OrderID, "order_sn": r.OrderSN,
			"order_type": r.OrderType, "order_amount": r.OrderAmount, "refund_amount": r.RefundAmount,
			"refund_type": r.RefundType, "transaction_id": r.TransactionID, "refund_way": r.RefundWay,
			"refund_status": r.RefundStatus, "create_time": util.FormatDateTime(r.CreateTime),
			"nickname": r.Nickname, "avatar": filesvc.GetFileURL(c, r.Avatar),
			"refund_type_text":   util.RefundTypeText(r.RefundType),
			"refund_status_text": util.RefundStatusText(r.RefundStatus),
			"refund_way_text":    util.RefundWayText(r.RefundWay),
		})
	}
	type extRow struct {
		Total   int64 `gorm:"column:total"`
		Ing     int64 `gorm:"column:ing"`
		Success int64 `gorm:"column:success"`
		Error   int64 `gorm:"column:error"`
	}
	var ext extRow
	extendWhere(tdb(c)).Select("count(r.id) as total, count(IF(r.refund_status=0,1,null)) as ing, count(IF(r.refund_status=1,1,null)) as success, count(IF(r.refund_status=2,1,null)) as error").Scan(&ext)
	response.Lists(c, out, count, q.PageNo, q.PageSize, gin.H{
		"total": ext.Total, "ing": ext.Ing, "success": ext.Success, "error": ext.Error,
	})
}

func OAReplyIndex(c *gin.Context) {
	appID, _, token := wechat.OAConfig(c)
	aesKey := cfgsvc.GetString(c, "oa_setting", "encoding_aes_key", "")
	encType := cfgsvc.GetInt(c, "oa_setting", "encryption_type", 1)
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
	msg, err := wechat.DecodeOABody(raw, token, aesKey, appID, encType, c.Query("msg_signature"), ts, nonce)
	if err != nil || msg.MsgType == "" {
		c.String(200, "success")
		return
	}
	q := tdb(c).Where("delete_time IS NULL AND status = 1")
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
	xmlBody := wechat.TextReplyXML(msg.FromUserName, msg.ToUserName, content)
	if encType >= 2 && aesKey != "" && xmlBody != "success" {
		if enc, err := wechat.EncryptedReplyXML(token, aesKey, appID, ts, nonce, xmlBody); err == nil {
			xmlBody = enc
		}
	}
	c.Header("Content-Type", "application/xml; charset=utf-8")
	c.String(200, xmlBody)
}
