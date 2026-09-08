package platformapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/tenantmenu"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TenantLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL")
	if lists.PHPTruthy(q, "keyword") {
		kw := lists.Param(q, "keyword")
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR sn LIKE ? OR tel LIKE ? OR domain_alias LIKE ?", like, like, like, like)
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
	var rows []model.Tenant
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	root := rootDomain(c)
	httpPrefix := "http://"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		httpPrefix = "https://"
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		users := tenantUserCount(t)
		def := httpPrefix + t.SN + "." + root + "/admin/"
		domain := def
		if t.DomainAliasEnable == 0 {
			domain = httpPrefix + t.DomainAlias + "/admin/"
		}
		avatar := t.Avatar
		if avatar == "" {
			avatar = firstNonEmpty(config.C.Project.Tenant["admin_avatar"], config.C.Project.Website["shop_logo"])
		}
		out = append(out, map[string]any{
			"id": t.ID, "sn": t.SN, "name": t.Name,
			"avatar": filesvc.GetFileURL(c, avatar), "disable": t.Disable,
			"create_time":  util.FormatDateTime(t.CreateTime),
			"update_time":  util.FormatDateTimeOrNil(t.UpdateTime),
			"delete_time":  util.FormatDateTimeOrNil(t.DeleteTime),
			"tactics":      t.Tactics,
			"domain_alias": t.DomainAlias, "domain_alias_enable": t.DomainAliasEnable,
			"notes": t.Notes, "tel": t.Tel, "users_count": users,
			"default_domain": def, "domain": domain,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func TenantDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "请选择用户")
		return
	}
	var t model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")).First(&t).Error != nil {
		response.Fail(c, "租户不存在")
		return
	}
	users := tenantUserCount(t)
	root := rootDomain(c)
	httpPrefix := "http://"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		httpPrefix = "https://"
	}
	def := httpPrefix + t.SN + "." + root + "/admin/"
	domain := def
	if t.DomainAliasEnable == 0 {
		domain = httpPrefix + t.DomainAlias + "/admin/"
	}
	avatar := t.Avatar
	if avatar == "" {
		avatar = firstNonEmpty(config.C.Project.Tenant["admin_avatar"], config.C.Project.Website["shop_logo"])
	}
	response.Success(c, "获取成功", gin.H{
		"id": t.ID, "sn": t.SN, "name": t.Name, "avatar": filesvc.GetFileURL(c, avatar),
		"tel": t.Tel, "domain_alias": t.DomainAlias, "domain_alias_enable": t.DomainAliasEnable,
		"disable": t.Disable, "create_time": util.FormatDateTime(t.CreateTime), "notes": t.Notes,
		"user_total": users, "default_domain": def, "domain": domain,
	})
}

func TenantAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	name := httpx.BodyRaw(c, "name")
	if !util.PHPRequired(httpx.Body(c), "name") {
		response.Fail(c, "请输入用户名")
		return
	}
	alias := stripHost(httpx.BodyStr(c, "domain_alias"))
	if tenantAliasTaken(alias, 0) {
		response.Fail(c, "租户别名已存在")
		return
	}
	sn := httpx.BodyStr(c, "host_name")
	if sn == "" {
		sn = randomSN()
	}
	var exist model.Tenant
	if bootstrap.DB.Where("sn = ? AND delete_time IS NULL", sn).First(&exist).Error == nil {
		response.Fail(c, "主机名已被占用，请更换")
		return
	}
	tactics := httpx.BodyInt(c, "tactics")
	now := util.NowUnix()
	tenant := model.Tenant{
		SN: sn, Name: name, Avatar: filesvc.SetFileURL(c, httpx.BodyStr(c, "avatar")),
		Tel: httpx.BodyStr(c, "tel"), DomainAlias: alias, DomainAliasEnable: httpx.BodyInt(c, "domain_alias_enable"),
		Disable: httpx.BodyInt(c, "disable"), Notes: httpx.BodyStr(c, "notes"), Tactics: tactics, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}
		if tactics == 1 {
			if err := runTenantSQL(tenant.SN); err != nil {
				return err
			}
			return initShardedTenant(tx, tenant, c)
		}
		return initSharedTenant(tx, tenant, c)
	})
	if err != nil {
		if tactics == 1 && sn != "" {
			dropShardedTenantTables(sn)
		}
		response.Fail(c, "新增失败："+err.Error())
		return
	}
	response.Result(c, 1, 1, "新增成功", []any{})
}

func TenantEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "请选择用户")
		return
	}
	id := httpx.BodyUint(c, "id")
	var cur model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "租户不存在")
		return
	}
	if !util.PHPRequired(httpx.Body(c), "name") {
		response.Fail(c, "请输入用户名")
		return
	}
	alias := stripHost(httpx.BodyStr(c, "domain_alias"))
	if tenantAliasTaken(alias, id) {
		response.Fail(c, "租户别名已存在")
		return
	}
	now := util.NowUnix()
	disable := httpx.BodyInt(c, "disable")
	bootstrap.DB.Model(&model.Tenant{}).Where("id = ? AND delete_time IS NULL", id).Updates(map[string]any{
		"name": httpx.BodyRaw(c, "name"), "avatar": filesvc.SetFileURL(c, httpx.BodyStr(c, "avatar")),
		"disable": disable, "tel": httpx.BodyStr(c, "tel"),
		"domain_alias":        alias,
		"domain_alias_enable": httpx.BodyInt(c, "domain_alias_enable"),
		"notes":               httpx.BodyStr(c, "notes"), "update_time": now,
	})
	if disable == 1 {
		expireTenantAdmins(cur)
	}
	response.Result(c, 1, 1, "操作成功", []any{})
}

func TenantDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "请选择用户")
		return
	}
	id := httpx.BodyUint(c, "id")
	var cur model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "租户不存在")
		return
	}
	expireTenantAdmins(cur)
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Tenant{}).Where("id = ? AND delete_time IS NULL", id).Updates(util.SoftDeleteFields(now))
	if cur.Tactics == 1 && cur.SN != "" {
		dropShardedTenantTables(cur.SN)
	}
	cleanTenantScopedRows(cur.ID)
	response.Result(c, 1, 1, "删除成功", []any{})
}

func tenantAdminDB(tenant model.Tenant) *gorm.DB {
	if tenant.Tactics == 1 && tenant.SN != "" {
		return tenantdb.UseSN(tenant.SN)
	}
	return bootstrap.DB
}

func dropShardedTenantTables(sn string) {
	if !validTenantSN(sn) {
		return
	}
	prefix := config.Prefix()
	for _, name := range tenantdb.ShardableNames() {
		table := prefix + name + "_" + sn
		_ = bootstrap.DB.Exec("DROP TABLE IF EXISTS `" + table + "`").Error
	}
}

func validTenantSN(sn string) bool {
	if sn == "" || len(sn) > 32 {
		return false
	}
	for _, r := range sn {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func expireTenantAdmins(tenant model.Tenant) {
	if tenant.ID == 0 {
		return
	}
	adb := tenantAdminDB(tenant)
	var admins []model.TenantAdmin
	adb.Where("tenant_id = ?", tenant.ID).Find(&admins)
	now := util.NowUnix()
	for _, a := range admins {
		var sess []model.TenantAdminSession
		adb.Where("admin_id = ?", a.ID).Find(&sess)
		for _, s := range sess {
			adb.Model(&s).Updates(map[string]any{"expire_time": now, "update_time": now})
			cache.DeleteTenantAdminInfo(s.Token)
		}
		cache.ClearAdminAuthCache(a.ID)
	}
	var users []model.User
	adb.Where("tenant_id = ?", tenant.ID).Find(&users)
	for _, u := range users {
		var sess []model.UserSession
		adb.Where("user_id = ?", u.ID).Find(&sess)
		for _, s := range sess {
			adb.Model(&s).Updates(map[string]any{"expire_time": now, "update_time": now})
			cache.DeleteUserInfo(s.Token)
		}
	}
}

// cleanTenantScopedRows removes leftover shared-schema rows after a tenant is deleted.
// tactics=1 shard tables are DROPped separately; this still clears shared tables
// (recharge/refund/OA reply/hot search/collect) and leftover tactics=0 copies.
func cleanTenantScopedRows(tid uint) {
	if tid == 0 || bootstrap.DB == nil {
		return
	}
	now := util.NowUnix()
	db := bootstrap.DB
	var adminIDs []uint
	db.Model(&model.TenantAdmin{}).Where("tenant_id = ?", tid).Pluck("id", &adminIDs)
	if len(adminIDs) > 0 {
		db.Where("admin_id IN ?", adminIDs).Delete(&model.TenantAdminRole{})
		db.Where("admin_id IN ?", adminIDs).Delete(&model.TenantAdminDept{})
		db.Where("admin_id IN ?", adminIDs).Delete(&model.TenantAdminJobs{})
		db.Where("admin_id IN ?", adminIDs).Delete(&model.TenantAdminSession{})
	}
	var roleIDs []uint
	db.Model(&model.TenantSystemRole{}).Where("tenant_id = ?", tid).Pluck("id", &roleIDs)
	if len(roleIDs) > 0 {
		db.Where("role_id IN ?", roleIDs).Delete(&model.TenantSystemRoleMenu{})
	}
	soft := []any{
		&model.TenantAdmin{}, &model.TenantDept{}, &model.TenantJobs{},
		&model.TenantFile{}, &model.TenantFileCate{}, &model.TenantSystemRole{},
		&model.User{}, &model.UserAccountLog{}, &model.Article{}, &model.ArticleCate{},
		&model.ArticleCollect{}, &model.OfficialAccountReply{}, &model.RechargeOrder{},
		&model.TenantSmsLog{}, &model.SmsLog{},
	}
	for _, m := range soft {
		db.Model(m).Where("tenant_id = ? AND delete_time IS NULL", tid).Updates(util.SoftDeleteFields(now))
	}
	hard := []any{
		&model.TenantConfig{}, &model.TenantPayConfig{}, &model.TenantPayWay{},
		&model.TenantNoticeSetting{}, &model.TenantNoticeRecord{},
		&model.TenantSystemMenu{}, &model.DecoratePage{}, &model.DecorateTabbar{},
		&model.HotSearch{}, &model.UserAuth{}, &model.UserSession{}, &model.RefundRecord{},
		&model.RefundLog{},
	}
	for _, m := range hard {
		db.Where("tenant_id = ?", tid).Delete(m)
	}
}

func TenantAdminLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	// PHP TenantAdminLists uses $this->params['tenant_id'] from request()->param().
	if lists.Param(q, "tenant_id") == "" {
		response.Lists(c, []any{}, 0, q.PageNo, q.PageSize, nil)
		return
	}
	tid := lists.ParamInt(q, "tenant_id")
	var tenant model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", tid).First(&tenant).Error != nil {
		response.Lists(c, []any{}, 0, q.PageNo, q.PageSize, nil)
		return
	}
	db := tenantAdminDB(tenant).Model(&model.TenantAdmin{}).Where("delete_time IS NULL AND tenant_id = ?", tid)
	if lists.PHPTruthy(q, "keyword") {
		kw := lists.Param(q, "keyword")
		db = db.Where("name LIKE ? OR account LIKE ?", "%"+kw+"%", "%"+kw+"%")
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
	var rows []model.TenantAdmin
	db.Order("create_time desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "root": a.Root, "name": a.Name,
			"avatar": filesvc.GetFileURL(c, firstNonEmpty(a.Avatar, config.C.Project.Tenant["admin_avatar"])), "account": a.Account,
			"multipoint_login": a.MultipointLogin, "disable": a.Disable,
			"create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

// tenantAdminIDExists mirrors PHP TenantAdminValidate::checkUser.
// Platform middleware copies request tenant_id onto the model scope, so
// tactics=1 admins are resolved on la_tenant_admin_{sn}, not the shared table.
func tenantAdminIDExists(id, tenantID uint) bool {
	_, _, ok := resolveTenantAdmin(tenantID, id)
	return ok
}

func TenantAdminDetail(c *gin.Context) {
	p := httpx.Query(c)
	if !util.PHPRequired(p, "id") {
		response.Fail(c, "请选择用户")
		return
	}
	// PHP sceneDetail is id.require|checkUser then tenant_id.require;
	// ThinkPHP require treats 0 as present, so id=0 hits checkUser first.
	if !tenantAdminIDExists(httpx.QueryUint(c, "id"), httpx.QueryUint(c, "tenant_id")) {
		response.Fail(c, "租户管理员不存在")
		return
	}
	if !util.PHPRequired(p, "tenant_id") {
		response.Fail(c, "请选择对应的租户")
		return
	}
	var tenant model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "tenant_id")).First(&tenant).Error != nil {
		response.Fail(c, "对应租户账号不存在")
		return
	}
	adb := tenantAdminDB(tenant)
	var a model.TenantAdmin
	tid := httpx.QueryUint(c, "tenant_id")
	q := adb.Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id"))
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&a).Error != nil {
		response.Fail(c, "租户管理员不存在")
		return
	}
	response.Success(c, "获取成功", gin.H{
		"id": a.ID, "root": a.Root, "name": a.Name,
		"avatar":  filesvc.GetFileURL(c, firstNonEmpty(a.Avatar, config.C.Project.Tenant["admin_avatar"])),
		"account": a.Account, "multipoint_login": a.MultipointLogin, "disable": a.Disable,
		"create_time": util.FormatDateTime(a.CreateTime),
	})
}

func TenantAdminAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	var tenant model.Tenant
	if msg := util.TenantAdminAddCheckTaken(p, func(tid uint) bool {
		return bootstrap.DB.Where("id = ? AND delete_time IS NULL", tid).First(&tenant).Error == nil
	}, func(tid uint, account string) bool {
		var exist model.TenantAdmin
		return tenantAdminDB(tenant).Where("account = ? AND tenant_id = ? AND delete_time IS NULL", account, tid).First(&exist).Error == nil
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	tid := httpx.BodyUint(c, "tenant_id")
	adb := tenantAdminDB(tenant)
	account := httpx.BodyRaw(c, "account")
	avatar := filesvc.SetFileURL(c, httpx.BodyStr(c, "avatar"))
	if avatar == "" {
		avatar = config.C.Project.DefaultImage["admin_avatar"]
	}
	now := util.NowUnix()
	admin := model.TenantAdmin{
		TenantID: tid, Account: account, Name: httpx.BodyRaw(c, "name"),
		Password: util.CreatePassword(httpx.BodyRaw(c, "password"), config.C.Project.UniqueIdentification),
		Disable:  httpx.BodyInt(c, "disable"), MultipointLogin: httpx.BodyInt(c, "multipoint_login"),
		Avatar: avatar, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	roles, depts, jobs := httpx.BodyUints(c, "role_id"), httpx.BodyUints(c, "dept_id"), httpx.BodyUints(c, "jobs_id")
	if msg := tenantAdminLinksCheck(adb, tid, roles, depts, jobs); msg != "" {
		response.Fail(c, msg)
		return
	}
	err := adb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		return saveTenantAdminLinks(tx, admin.ID, roles, depts, jobs)
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	cache.ClearAdminAuthCache(admin.ID)
	response.SuccessNotice(c, "操作成功")
}

func TenantAdminEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !util.PHPRequired(p, "id") {
		response.Fail(c, "请选择用户")
		return
	}
	if !tenantAdminIDExists(httpx.BodyUint(c, "id"), httpx.BodyUint(c, "tenant_id")) {
		response.Fail(c, "租户管理员不存在")
		return
	}
	if msg := util.TenantAdminEditCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.BodyUint(c, "id")
	var tenant model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "tenant_id")).First(&tenant).Error != nil {
		response.Fail(c, "对应租户账号不存在")
		return
	}
	adb := tenantAdminDB(tenant)
	var a model.TenantAdmin
	aq := adb.Where("id = ? AND delete_time IS NULL", id)
	if tenant.ID > 0 {
		aq = aq.Where("tenant_id = ?", tenant.ID)
	}
	if aq.First(&a).Error != nil {
		response.Fail(c, "租户管理员不存在")
		return
	}
	if a.Root == 1 && httpx.BodyInt(c, "disable") == 1 {
		response.Fail(c, "超级管理员不允许被禁用")
		return
	}
	if account := httpx.BodyRaw(c, "account"); account != "" && account != "0" {
		var taken model.TenantAdmin
		tq := adb.Where("account = ? AND delete_time IS NULL AND id <> ?", account, id)
		if tenant.ID > 0 {
			tq = tq.Where("tenant_id = ?", tenant.ID)
		}
		if tq.First(&taken).Error == nil {
			response.Fail(c, "账号已存在")
			return
		}
	}
	now := util.NowUnix()
	data := map[string]any{
		"name":             httpx.BodyRaw(c, "name"),
		"account":          httpx.BodyRaw(c, "account"),
		"disable":          httpx.BodyInt(c, "disable"),
		"multipoint_login": httpx.BodyInt(c, "multipoint_login"),
		"update_time":      now,
	}
	// PHP TenantAdminLogic::edit always writes avatar: empty() → ''.
	if av := httpx.BodyStr(c, "avatar"); av != "" && av != "0" {
		data["avatar"] = filesvc.SetFileURL(c, av)
	} else {
		data["avatar"] = ""
	}
	if pwd := httpx.BodyRaw(c, "password"); pwd != "" && pwd != "0" {
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	var oldRoles []uint
	adb.Model(&model.TenantAdminRole{}).Where("admin_id = ?", id).Pluck("role_id", &oldRoles)
	newRoles := httpx.BodyUints(c, "role_id")
	depts, jobs := httpx.BodyUints(c, "dept_id"), httpx.BodyUints(c, "jobs_id")
	if msg := tenantAdminLinksCheck(adb, tenant.ID, newRoles, depts, jobs); msg != "" {
		response.Fail(c, msg)
		return
	}
	err := adb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TenantAdmin{}).Where("id = ? AND delete_time IS NULL", id).Updates(data).Error; err != nil {
			return err
		}
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminRole{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminDept{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminJobs{})
		return saveTenantAdminLinks(tx, id, newRoles, depts, jobs)
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	if httpx.BodyInt(c, "disable") == 1 || util.UintSlicesChanged(oldRoles, newRoles) {
		expireTenantAdminTokens(adb, id)
	}
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func TenantAdminDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !util.PHPRequired(httpx.Body(c), "id") {
		response.Fail(c, "请选择用户")
		return
	}
	id := httpx.BodyUint(c, "id")
	adb, a, ok := resolveTenantAdmin(httpx.BodyUint(c, "tenant_id"), id)
	if !ok {
		response.Fail(c, "租户管理员不存在")
		return
	}
	if a.Root == 1 {
		response.Fail(c, "超级管理员不允许被删除")
		return
	}
	err := adb.Transaction(func(tx *gorm.DB) error {
		now := util.NowUnix()
		q := tx.Model(&model.TenantAdmin{}).Where("id = ? AND delete_time IS NULL", id)
		if a.TenantID > 0 {
			q = q.Where("tenant_id = ?", a.TenantID)
		}
		if err := q.Updates(util.SoftDeleteFields(now)).Error; err != nil {
			return err
		}
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminRole{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminDept{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminJobs{})
		return nil
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	expireTenantAdminTokens(adb, id)
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "删除成功")
}

func resolveTenantAdmin(tid, adminID uint) (*gorm.DB, model.TenantAdmin, bool) {
	if tid > 0 {
		var tenant model.Tenant
		if bootstrap.DB.Where("id = ? AND delete_time IS NULL", tid).First(&tenant).Error != nil {
			return bootstrap.DB, model.TenantAdmin{}, false
		}
		adb := tenantAdminDB(tenant)
		var a model.TenantAdmin
		q := adb.Where("id = ? AND delete_time IS NULL", adminID)
		if tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		if q.First(&a).Error != nil {
			return adb, a, false
		}
		return adb, a, true
	}
	var a model.TenantAdmin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", adminID).First(&a).Error == nil {
		if a.TenantID > 0 {
			var tenant model.Tenant
			if bootstrap.DB.Where("id = ? AND delete_time IS NULL", a.TenantID).First(&tenant).Error == nil {
				return tenantAdminDB(tenant), a, true
			}
		}
		return bootstrap.DB, a, true
	}
	// PHP TenantAdmin::findOrEmpty($id) only hits the shared table. Scanning
	// every la_tenant_admin_{sn} by id alone can soft-delete another tenant's
	// admin when AUTO_INCREMENT ids collide (each shard typically has id=1).
	return bootstrap.DB, a, false
}

func tenantAdminLinksCheck(db *gorm.DB, tid uint, roles, depts, jobs []uint) string {
	if !tenantLinkIDsOwned(db, tid, &model.TenantSystemRole{}, roles, "delete_time IS NULL") {
		return "角色不存在"
	}
	if !tenantLinkIDsOwned(db, tid, &model.TenantDept{}, depts, "delete_time IS NULL") {
		return "部门不存在"
	}
	if !tenantLinkIDsOwned(db, tid, &model.TenantJobs{}, jobs, "delete_time IS NULL") {
		return "岗位不存在"
	}
	return ""
}

func tenantLinkIDsOwned(db *gorm.DB, tid uint, dest any, ids []uint, extra string) bool {
	seen := map[uint]struct{}{}
	uniq := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return true
	}
	q := db.Model(dest).Where("id IN ?", uniq)
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if extra != "" {
		q = q.Where(extra)
	}
	var n int64
	q.Count(&n)
	return n == int64(len(uniq))
}

func saveTenantAdminLinks(tx *gorm.DB, adminID uint, roles, depts, jobs []uint) error {
	for _, id := range roles {
		if err := tx.Create(&model.TenantAdminRole{AdminID: adminID, RoleID: id}).Error; err != nil {
			return err
		}
	}
	for _, id := range depts {
		if err := tx.Create(&model.TenantAdminDept{AdminID: adminID, DeptID: id}).Error; err != nil {
			return err
		}
	}
	for _, id := range jobs {
		if err := tx.Create(&model.TenantAdminJobs{AdminID: adminID, JobsID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

func expireTenantAdminTokens(db *gorm.DB, adminID uint) {
	if db == nil {
		db = bootstrap.DB
	}
	var sess []model.TenantAdminSession
	db.Where("admin_id = ?", adminID).Find(&sess)
	now := util.NowUnix()
	for _, s := range sess {
		db.Model(&s).Updates(map[string]any{"expire_time": now, "update_time": now})
		cache.DeleteTenantAdminInfo(s.Token)
	}
}

func TenantUserLists(c *gin.Context) {
	// PHP TenantUserController::lists validates sceneManager before dataLists,
	// so a POST without query tenant_id is 请选择租户标识, not 请求方式错误.
	tid := httpx.QueryInt(c, "tenant_id")
	if tid <= 0 {
		response.Fail(c, "请选择租户标识")
		return
	}
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := tenantdb.ForTenant(uint(tid))
	if db == nil {
		db = bootstrap.DB
	}
	db = db.Model(&model.User{}).Where("delete_time IS NULL")
	if tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if lists.PHPTruthy(q, "keyword") {
		kw := lists.Param(q, "keyword")
		like := "%" + kw + "%"
		db = db.Where("sn LIKE ? OR nickname LIKE ? OR account LIKE ? OR mobile LIKE ?", like, like, like, like)
	}
	if lists.PHPTruthy(q, "channel") {
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
			"id": u.ID, "sn": u.SN, "avatar": filesvc.GetFileURL(c, u.Avatar),
			"nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
			"sex": util.SexDesc(u.Sex), "channel": util.ChannelDesc(u.Channel), "is_disable": u.IsDisable,
			"create_time": util.FormatDateTime(u.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func TenantUserDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "请选择用户")
		return
	}
	if !util.PHPRequired(httpx.Query(c), "tenant_id") {
		response.Fail(c, "请选择租户标识")
		return
	}
	tid := httpx.QueryUint(c, "tenant_id")
	var u model.User
	q := tenantdb.ForTenant(tid).Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id"))
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&u).Error != nil {
		response.Fail(c, "用户不存在！")
		return
	}
	response.Success(c, "获取租户用户详情成功", userMap(c, u))
}

func initSharedTenant(tx *gorm.DB, tenant model.Tenant, c *gin.Context) error {
	// PHP TenantAdminLogic::initialization uses ?: on raw params.
	pwd := httpx.BodyRaw(c, "password")
	if pwd == "" || pwd == "0" {
		pwd = config.C.Project.DefaultPassword
	}
	account := httpx.BodyRaw(c, "account")
	if account == "" || account == "0" {
		account = tenant.SN
	}
	now := util.NowUnix()
	admin := newTenantSuperAdmin(0, tenant.ID, account, util.CreatePassword(pwd, config.C.Project.UniqueIdentification), now)
	if err := tx.Create(&admin).Error; err != nil {
		return err
	}
	if err := copyTenantDept(tx, tenant.ID, admin.ID); err != nil {
		return err
	}
	if err := copyTenantMenus(tx, tenant.ID); err != nil {
		return err
	}
	if err := copyTenantArticles(tx, tenant.ID); err != nil {
		return err
	}
	if err := copyTenantPay(tx, tenant.ID); err != nil {
		return err
	}
	if err := copyTenantNotice(tx, tenant.ID); err != nil {
		return err
	}
	return copyTenantDecorate(tx, tenant.ID)
}

func initShardedTenant(tx *gorm.DB, tenant model.Tenant, c *gin.Context) error {
	_ = tx
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
	}
	if err := runTenantDataSQL(tenant.ID, tenant.SN); err != nil {
		return err
	}
	sdb := tenantdb.UseSN(tenant.SN)
	// PHP TenantCreatService::initAccount uses isset() on raw params.
	pwd := config.C.Project.DefaultPassword
	if httpx.BodyHas(c, "password") {
		pwd = httpx.BodyRaw(c, "password")
	}
	account := "admin"
	if httpx.BodyHas(c, "account") {
		account = httpx.BodyRaw(c, "account")
	}
	now := util.NowUnix()
	admin := newTenantSuperAdmin(1, tenant.ID, account, util.CreatePassword(pwd, config.C.Project.UniqueIdentification), now)
	if err := sdb.Create(&admin).Error; err != nil {
		return err
	}
	if err := copyTenantNotice(sdb, tenant.ID); err != nil {
		return err
	}
	return sdb.Create(&model.TenantAdminDept{AdminID: 1, DeptID: 1}).Error
}

func copyTenantDept(tx *gorm.DB, tenantID, adminID uint) error {
	var tpl model.TenantDept
	if err := tx.Where("tenant_id = 0 AND delete_time IS NULL").First(&tpl).Error; err != nil {
		return fmt.Errorf("部门模板缺失")
	}
	now := util.NowUnix()
	dept := model.TenantDept{
		Name: tpl.Name, Pid: 0, Sort: tpl.Sort, Leader: tpl.Leader, Mobile: tpl.Mobile,
		Status: tpl.Status, TenantID: tenantID, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if err := tx.Create(&dept).Error; err != nil {
		return err
	}
	return tx.Create(&model.TenantAdminDept{AdminID: adminID, DeptID: dept.ID}).Error
}

// copyTenantArticles remaps each template category once. PHP ArticleLogic::initialization
// nests cate create inside the article loop (duplicate cate rows per article); Go keeps
// one cate per template id so tenant article/cid graphs stay consistent.
func copyTenantArticles(tx *gorm.DB, tenantID uint) error {
	var cates []model.ArticleCate
	tx.Where("tenant_id = 0 AND delete_time IS NULL").Find(&cates)
	idMap := map[uint]uint{}
	now := util.NowUnix()
	for _, cate := range cates {
		old := cate.ID
		cate.ID = 0
		cate.TenantID = tenantID
		cate.CreateTime = now
		cate.UpdateTime = util.UnixPtr(now)
		if err := tx.Create(&cate).Error; err != nil {
			return err
		}
		idMap[old] = cate.ID
	}
	var arts []model.Article
	tx.Where("tenant_id = 0 AND delete_time IS NULL").Find(&arts)
	for _, a := range arts {
		a.ID = 0
		a.TenantID = tenantID
		if nid, ok := idMap[a.Cid]; ok {
			a.Cid = nid
		}
		a.CreateTime = now
		a.UpdateTime = util.UnixPtr(now)
		if err := tx.Create(&a).Error; err != nil {
			return err
		}
	}
	return nil
}

func copyTenantPay(tx *gorm.DB, tenantID uint) error {
	var tpls []model.TenantPayConfig
	tx.Where("tenant_id = 0").Find(&tpls)
	oldToNew := map[uint]uint{}
	wayToID := map[int]uint{}
	for _, cfg := range tpls {
		oldID, payWay := cfg.ID, cfg.PayWay
		cfg.ID = 0
		cfg.TenantID = tenantID
		if err := tx.Create(&cfg).Error; err != nil {
			return err
		}
		oldToNew[oldID] = cfg.ID
		wayToID[payWay] = cfg.ID
	}
	var ways []model.TenantPayWay
	tx.Where("tenant_id = 0").Find(&ways)
	for _, w := range ways {
		w.ID = 0
		w.TenantID = tenantID
		w.PayConfigID = remapTenantPayConfigID(w.PayConfigID, oldToNew, wayToID, w.Scene)
		if err := tx.Create(&w).Error; err != nil {
			return err
		}
	}
	return nil
}

// remapTenantPayConfigID prefers the copied config row id, then PHP's
// pay_config_id==pay_way match, then scene when the template id is empty.
func remapTenantPayConfigID(payConfigID uint, oldToNew map[uint]uint, wayToID map[int]uint, scene int) uint {
	if nid, ok := oldToNew[payConfigID]; ok {
		return nid
	}
	if nid, ok := wayToID[int(payConfigID)]; ok {
		return nid
	}
	if payConfigID == 0 {
		if nid, ok := wayToID[scene]; ok {
			return nid
		}
	}
	return payConfigID
}

func copyTenantNotice(dest *gorm.DB, tenantID uint) error {
	src := bootstrap.DB
	if src == nil {
		src = dest
	}
	var tpls []model.TenantNoticeSetting
	if err := src.Where("tenant_id = 0").Find(&tpls).Error; err != nil {
		return err
	}
	now := util.NowUnix()
	for _, n := range tpls {
		n.ID = 0
		n.TenantID = tenantID
		n.UpdateTime = util.UnixPtr(now)
		n.SystemNotice = canonicalizeNoticeJSON(n.SystemNotice)
		n.SmsNotice = canonicalizeNoticeJSON(n.SmsNotice)
		n.OaNotice = canonicalizeNoticeJSON(n.OaNotice)
		n.MnpNotice = canonicalizeNoticeJSON(n.MnpNotice)
		if err := dest.Create(&n).Error; err != nil {
			return err
		}
	}
	return nil
}

// canonicalizeNoticeJSON mirrors PHP NoticeLogic::initialization re-encoding
// after ThinkPHP JSON getters so copied rows stay valid JSON objects.
func canonicalizeNoticeJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	var v any = raw
	for i := 0; i < 3; i++ {
		s, ok := v.(string)
		if !ok {
			break
		}
		var next any
		if json.Unmarshal([]byte(s), &next) != nil {
			return s
		}
		v = next
	}
	return util.EncodeJSON(v)
}

func copyTenantDecorate(tx *gorm.DB, tenantID uint) error {
	now := util.NowUnix()
	var pages []model.DecoratePage
	tx.Where("tenant_id = 0").Find(&pages)
	for _, p := range pages {
		p.ID = 0
		p.TenantID = tenantID
		p.CreateTime = now
		p.UpdateTime = util.UnixPtr(now)
		if err := tx.Create(&p).Error; err != nil {
			return err
		}
	}
	var bars []model.DecorateTabbar
	tx.Where("tenant_id = 0").Find(&bars)
	for _, b := range bars {
		b.ID = 0
		b.TenantID = tenantID
		b.CreateTime = now
		b.UpdateTime = util.UnixPtr(now)
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
	}
	return nil
}

func copyTenantMenus(tx *gorm.DB, tenantID uint) error {
	return tenantmenu.Reinit(tx, tx, tenantID)
}

func runTenantDataSQL(tenantID uint, sn string) error {
	candidates := []string{
		filepath.Join(config.C.App.PublicDir, "../app/platformapi/db/tenantData.sql"),
		"/workspace/server/app/platformapi/db/tenantData.sql",
	}
	var raw []byte
	var err error
	for _, p := range candidates {
		raw, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	content := applyTenantSQLPlaceholders(string(raw), sn, tenantID)
	return bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		return execSQLScript(tx, content)
	})
}

// applyTenantSQLPlaceholders mirrors PHP TenantCreatService prefix + token rewrite.
func applyTenantSQLPlaceholders(content, sn string, tenantID uint) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "{tenantSn}", sn)
	if tenantID > 0 {
		content = strings.ReplaceAll(content, "{tenantId}", util.ToString(tenantID))
	}
	content = strings.ReplaceAll(content, "`la_", tenantSQLDatabase()+".`la_")
	content = strings.ReplaceAll(content, "`la_", "`"+config.Prefix())
	return content
}

// tenantSQLDatabase mirrors PHP env('database.database', 'likeadmin_saas')
// used when TenantCreatService qualifies `la_*` as db.`la_*.
func tenantSQLDatabase() string {
	if db := strings.TrimSpace(config.C.Database.Database); db != "" {
		return db
	}
	return "likeadmin_saas"
}

func execSQLScript(db *gorm.DB, content string) error {
	if db == nil {
		db = bootstrap.DB
	}
	parts := strings.Split(content, ";\n")
	for _, sql := range parts {
		sql = strings.TrimSpace(sql)
		if sql == "" || strings.HasPrefix(sql, "--") || strings.HasPrefix(sql, "/*") {
			continue
		}
		up := strings.ToUpper(sql)
		// BEGIN/COMMIT are no-ops: callers already wrap in a GORM transaction.
		if strings.HasPrefix(up, "BEGIN") || strings.HasPrefix(up, "COMMIT") || strings.HasPrefix(up, "ROLLBACK") {
			continue
		}
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

func runTenantSQL(sn string) error {
	candidates := []string{
		filepath.Join(config.C.App.PublicDir, "../app/platformapi/db/tenant.sql"),
		"/workspace/server/app/platformapi/db/tenant.sql",
	}
	var raw []byte
	var err error
	for _, p := range candidates {
		raw, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return err
	}
	return execSQLScript(bootstrap.DB, applyTenantSQLPlaceholders(string(raw), sn, 0))
}

func randomSN() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	for {
		b := make([]byte, 8)
		n := util.NowUnix()
		for i := 0; i < 8; i++ {
			b[i] = chars[int(n+int64(i*17))%len(chars)]
			n = n*1103515245 + 12345
		}
		sn := string(b)
		var t model.Tenant
		if bootstrap.DB.Where("sn = ?", sn).First(&t).Error != nil {
			return sn
		}
	}
}

func tenantUserCount(t model.Tenant) int64 {
	var users int64
	tenantdb.ForTenant(t.ID).Model(&model.User{}).Where("tenant_id = ? AND delete_time IS NULL", t.ID).Count(&users)
	return users
}

func stripHost(s string) string {
	re := regexp.MustCompile(`^https?://|/$`)
	return re.ReplaceAllString(s, "")
}

// tenantAliasTaken mirrors PHP TenantValidate::checkDomainAlias / checkDomainAliasEdit.
// ThinkPHP skips the custom rule when domain_alias is empty (no require).
func tenantAliasTaken(alias string, excludeID uint) bool {
	if alias == "" || bootstrap.DB == nil {
		return false
	}
	q := bootstrap.DB.Where("domain_alias = ? AND delete_time IS NULL", alias)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var row model.Tenant
	return q.First(&row).Error == nil
}

// newTenantSuperAdmin mirrors PHP TenantAdminLogic::initialization / initAccount:
// ThinkPHP create() and the sharded INSERT both write create_time and update_time = now.
func newTenantSuperAdmin(id, tenantID uint, account, password string, now int64) model.TenantAdmin {
	admin := model.TenantAdmin{
		TenantID: tenantID, Account: account, Name: "超级管理员",
		Password: password, Root: 1, MultipointLogin: 1,
		CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if id != 0 {
		admin.ID = id
	}
	return admin
}

func rootDomain(c *gin.Context) string {
	host := ctxutil.Host(c)
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	if i := strings.Index(host, ":"); i >= 0 {
		host = host[:i]
	}
	parts := strings.Split(host, ".")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], ".")
	}
	return host
}

func userMap(c *gin.Context, u model.User) map[string]any {
	return map[string]any{
		"id": u.ID, "sn": u.SN,
		"avatar":    filesvc.GetFileURL(c, u.Avatar),
		"real_name": u.RealName, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
		"sex": util.SexDesc(u.Sex), "sexCode": u.Sex, "channel": util.ChannelDesc(u.Channel),
		"is_disable": u.IsDisable,
		"login_time": util.FormatDateTimePtr(u.LoginTime), "user_money": util.MoneyString(u.UserMoney),
		"create_time": util.FormatDateTime(u.CreateTime),
	}
}
