package platformapi

import (
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
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TenantLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL")
	if kw := lists.Param(q, "keyword"); kw != "" {
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
		if t.DomainAliasEnable == 0 && t.DomainAlias != "" {
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
	var t model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&t).Error != nil {
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
	if t.DomainAliasEnable == 0 && t.DomainAlias != "" {
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
	name := httpx.Str(c, "name")
	if name == "" {
		response.Fail(c, "请输入用户名")
		return
	}
	alias := stripHost(httpx.Str(c, "domain_alias"))
	var aliasRow model.Tenant
	if bootstrap.DB.Where("domain_alias = ? AND delete_time IS NULL", alias).First(&aliasRow).Error == nil {
		response.Fail(c, "租户别名已存在")
		return
	}
	sn := httpx.Str(c, "host_name")
	if sn == "" {
		sn = randomSN()
	}
	var exist model.Tenant
	if bootstrap.DB.Where("sn = ? AND delete_time IS NULL", sn).First(&exist).Error == nil {
		response.Fail(c, "主机名已被占用，请更换")
		return
	}
	tactics := httpx.Int(c, "tactics")
	now := util.NowUnix()
	tenant := model.Tenant{
		SN: sn, Name: name, Avatar: filesvc.SetFileURL(c, httpx.Str(c, "avatar")),
		Tel: httpx.Str(c, "tel"), DomainAlias: alias, DomainAliasEnable: httpx.Int(c, "domain_alias_enable"),
		Disable: httpx.Int(c, "disable"), Notes: httpx.Str(c, "notes"), Tactics: tactics, CreateTime: now,
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
		response.Fail(c, "新增失败："+err.Error())
		return
	}
	response.Result(c, 1, 1, "新增成功", []any{})
}

func TenantEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "请选择用户")
		return
	}
	if httpx.Str(c, "name") == "" {
		response.Fail(c, "请输入用户名")
		return
	}
	var cur model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "租户不存在")
		return
	}
	alias := stripHost(httpx.Str(c, "domain_alias"))
	var aliasRow model.Tenant
	if bootstrap.DB.Where("domain_alias = ? AND id <> ? AND delete_time IS NULL", alias, id).First(&aliasRow).Error == nil {
		response.Fail(c, "租户别名已存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Tenant{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "avatar": filesvc.SetFileURL(c, httpx.Str(c, "avatar")),
		"disable": httpx.Int(c, "disable"), "tel": httpx.Str(c, "tel"),
		"domain_alias":        alias,
		"domain_alias_enable": httpx.Int(c, "domain_alias_enable"),
		"notes":               httpx.Str(c, "notes"), "update_time": now,
	})
	response.Result(c, 1, 1, "操作成功", []any{})
}

func TenantDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "请选择用户")
		return
	}
	var cur model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "租户不存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Tenant{}).Where("id = ?", id).Update("delete_time", now)
	if cur.Tactics == 1 && cur.SN != "" {
		dropShardedTenantTables(cur.SN)
	}
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

func TenantAdminLists(c *gin.Context) {
	q := lists.Parse(c)
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
	if kw := lists.Param(q, "keyword"); kw != "" {
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
			"avatar": filesvc.GetFileURL(c, a.Avatar), "account": a.Account,
			"multipoint_login": a.MultipointLogin, "disable": a.Disable,
			"create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func TenantAdminDetail(c *gin.Context) {
	p := httpx.Params(c)
	if !phpRequiredParam(p, "id") {
		response.Fail(c, "请选择用户")
		return
	}
	if !phpRequiredParam(p, "tenant_id") {
		response.Fail(c, "请选择对应的租户")
		return
	}
	var tenant model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "tenant_id")).First(&tenant).Error != nil {
		response.Fail(c, "对应租户账号不存在")
		return
	}
	adb := tenantAdminDB(tenant)
	var a model.TenantAdmin
	if adb.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&a).Error != nil {
		response.Fail(c, "租户管理员不存在")
		return
	}
	response.Success(c, "获取成功", gin.H{
		"id": a.ID, "root": a.Root, "name": a.Name, "avatar": filesvc.GetFileURL(c, a.Avatar),
		"account": a.Account, "multipoint_login": a.MultipointLogin, "disable": a.Disable,
		"create_time": util.FormatDateTime(a.CreateTime),
	})
}

func TenantAdminAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.TenantAdminAddCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	tid := httpx.Uint(c, "tenant_id")
	var tenant model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", tid).First(&tenant).Error != nil {
		response.Fail(c, "对应租户账号不存在")
		return
	}
	adb := tenantAdminDB(tenant)
	account := httpx.Str(c, "account")
	var exist model.TenantAdmin
	if adb.Where("account = ? AND tenant_id = ? AND delete_time IS NULL", account, tid).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	avatar := filesvc.SetFileURL(c, httpx.Str(c, "avatar"))
	if avatar == "" {
		avatar = config.C.Project.DefaultImage["admin_avatar"]
	}
	admin := model.TenantAdmin{
		TenantID: tid, Account: account, Name: httpx.Str(c, "name"),
		Password: util.CreatePassword(httpx.Str(c, "password"), config.C.Project.UniqueIdentification),
		Disable:  httpx.Int(c, "disable"), MultipointLogin: httpx.Int(c, "multipoint_login"),
		Avatar: avatar, CreateTime: util.NowUnix(),
	}
	err := adb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		return saveTenantAdminLinks(tx, admin.ID, httpx.Uints(c, "role_id"), httpx.Uints(c, "dept_id"), httpx.Uints(c, "jobs_id"))
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func TenantAdminEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.TenantAdminEditCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	var tenant model.Tenant
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "tenant_id")).First(&tenant).Error != nil {
		response.Fail(c, "对应租户账号不存在")
		return
	}
	adb := tenantAdminDB(tenant)
	var a model.TenantAdmin
	if adb.Where("id = ? AND delete_time IS NULL", id).First(&a).Error != nil {
		response.Fail(c, "租户管理员不存在")
		return
	}
	now := util.NowUnix()
	data := map[string]any{
		"name":             httpx.Str(c, "name"),
		"account":          httpx.Str(c, "account"),
		"disable":          httpx.Int(c, "disable"),
		"multipoint_login": httpx.Int(c, "multipoint_login"),
		"update_time":      now,
	}
	if avatar := httpx.Str(c, "avatar"); avatar != "" {
		data["avatar"] = filesvc.SetFileURL(c, avatar)
	} else if _, ok := p["avatar"]; ok {
		data["avatar"] = ""
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	var oldRoles []uint
	adb.Model(&model.TenantAdminRole{}).Where("admin_id = ?", id).Pluck("role_id", &oldRoles)
	newRoles := httpx.Uints(c, "role_id")
	err := adb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.TenantAdmin{}).Where("id = ?", id).Updates(data).Error; err != nil {
			return err
		}
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminRole{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminDept{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminJobs{})
		return saveTenantAdminLinks(tx, id, newRoles, httpx.Uints(c, "dept_id"), httpx.Uints(c, "jobs_id"))
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	if httpx.Int(c, "disable") == 1 || tenantAdminRolesChanged(oldRoles, newRoles) {
		expireTenantAdminTokens(adb, id)
	}
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func TenantAdminDelete(c *gin.Context) {
	if !phpRequiredParam(httpx.Params(c), "id") {
		response.Fail(c, "请选择用户")
		return
	}
	id := httpx.Uint(c, "id")
	adb, a, ok := resolveTenantAdmin(httpx.Uint(c, "tenant_id"), id)
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
		if err := tx.Model(&model.TenantAdmin{}).Where("id = ?", id).Update("delete_time", now).Error; err != nil {
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
		if adb.Where("id = ? AND delete_time IS NULL", adminID).First(&a).Error != nil {
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
	var tenants []model.Tenant
	bootstrap.DB.Where("tactics = 1 AND delete_time IS NULL AND sn <> ''").Find(&tenants)
	for _, t := range tenants {
		adb := tenantAdminDB(t)
		var row model.TenantAdmin
		if adb.Where("id = ? AND delete_time IS NULL", adminID).First(&row).Error == nil {
			return adb, row, true
		}
	}
	return bootstrap.DB, a, false
}

func phpRequiredParam(p map[string]any, key string) bool {
	v, ok := p[key]
	if !ok || v == nil {
		return false
	}
	s := strings.TrimSpace(util.ToString(v))
	return s != ""
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

func tenantAdminRolesChanged(oldRoles, newRoles []uint) bool {
	if len(oldRoles) != len(newRoles) {
		return true
	}
	seen := map[uint]int{}
	for _, id := range oldRoles {
		seen[id]++
	}
	for _, id := range newRoles {
		seen[id]--
		if seen[id] < 0 {
			return true
		}
	}
	for _, n := range seen {
		if n != 0 {
			return true
		}
	}
	return false
}

func TenantUserLists(c *gin.Context) {
	q := lists.Parse(c)
	tid := lists.ParamInt(q, "tenant_id")
	if tid <= 0 {
		response.Fail(c, "请选择租户标识")
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
			"id": u.ID, "sn": u.SN, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
			"real_name": u.RealName, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
			"sex": util.SexDesc(u.Sex), "channel": util.ChannelDesc(u.Channel), "is_disable": u.IsDisable, "user_money": util.MoneyString(u.UserMoney),
			"create_time": util.FormatDateTime(u.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func TenantUserDetail(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "请选择用户")
		return
	}
	tid := httpx.Uint(c, "tenant_id")
	if tid == 0 {
		tid = ctxutil.Get(c).TenantID
	}
	var u model.User
	if tenantdb.ForTenant(tid).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&u).Error != nil {
		response.Fail(c, "用户不存在")
		return
	}
	response.Success(c, "获取租户用户详情成功", userMap(c, u))
}

func initSharedTenant(tx *gorm.DB, tenant model.Tenant, c *gin.Context) error {
	pwd := httpx.Str(c, "password")
	if pwd == "" {
		pwd = config.C.Project.DefaultPassword
	}
	account := httpx.Str(c, "account")
	if account == "" {
		account = tenant.SN
	}
	admin := model.TenantAdmin{
		TenantID: tenant.ID, Account: account, Name: "超级管理员",
		Password: util.CreatePassword(pwd, config.C.Project.UniqueIdentification),
		Root:     1, MultipointLogin: 1, CreateTime: util.NowUnix(),
	}
	if err := tx.Create(&admin).Error; err != nil {
		return err
	}
	dept := model.TenantDept{Name: "公司", Pid: 0, Sort: 0, Status: 1, TenantID: tenant.ID, CreateTime: util.NowUnix()}
	if err := tx.Create(&dept).Error; err != nil {
		return err
	}
	_ = tx.Create(&model.TenantAdminDept{AdminID: admin.ID, DeptID: dept.ID}).Error
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
	if err := runTenantDataSQL(tenant.ID, tenant.SN); err != nil {
		return err
	}
	sdb := tenantdb.UseSN(tenant.SN)
	pwd := httpx.Str(c, "password")
	if pwd == "" {
		pwd = config.C.Project.DefaultPassword
	}
	account := httpx.Str(c, "account")
	if account == "" {
		account = "admin"
	}
	now := util.NowUnix()
	admin := model.TenantAdmin{
		ID: 1, TenantID: tenant.ID, Account: account, Name: "超级管理员",
		Password: util.CreatePassword(pwd, config.C.Project.UniqueIdentification),
		Root:     1, MultipointLogin: 1, CreateTime: now,
	}
	if err := sdb.Create(&admin).Error; err != nil {
		return err
	}
	if err := copyTenantNotice(sdb, tenant.ID); err != nil {
		return err
	}
	return sdb.Create(&model.TenantAdminDept{AdminID: 1, DeptID: 1}).Error
}

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
	for _, n := range tpls {
		n.ID = 0
		n.TenantID = tenantID
		if err := dest.Create(&n).Error; err != nil {
			return err
		}
	}
	return nil
}

func copyTenantDecorate(tx *gorm.DB, tenantID uint) error {
	now := util.NowUnix()
	var pages []model.DecoratePage
	tx.Where("tenant_id = 0").Find(&pages)
	for _, p := range pages {
		p.ID = 0
		p.TenantID = tenantID
		p.CreateTime = now
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
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
	}
	return nil
}

func copyTenantMenus(tx *gorm.DB, tenantID uint) error {
	var tpls []model.TenantSystemMenu
	tx.Where("tenant_id = 0").Order("pid, id").Find(&tpls)
	if len(tpls) == 0 {
		var plat []model.SystemMenu
		tx.Order("pid, id").Find(&plat)
		idMap := map[uint]uint{}
		for _, m := range plat {
			old := m.ID
			row := model.TenantSystemMenu{
				Pid: m.Pid, Type: m.Type, Name: m.Name, Icon: m.Icon, Sort: m.Sort, Perms: m.Perms,
				Paths: m.Paths, Component: m.Component, Selected: m.Selected, Params: m.Params,
				IsCache: m.IsCache, IsShow: m.IsShow, IsDisable: m.IsDisable, TenantID: tenantID,
				CreateTime: util.NowUnix(),
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			idMap[old] = row.ID
		}
		var created []model.TenantSystemMenu
		tx.Where("tenant_id = ?", tenantID).Find(&created)
		for _, item := range created {
			if item.Pid != 0 {
				if nid, ok := idMap[item.Pid]; ok {
					tx.Model(&item).Update("pid", nid)
				}
			}
		}
		return nil
	}
	idMap := map[uint]uint{}
	for _, m := range tpls {
		old := m.ID
		row := m
		row.ID = 0
		row.TenantID = tenantID
		row.CreateTime = util.NowUnix()
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		idMap[old] = row.ID
	}
	var created []model.TenantSystemMenu
	tx.Where("tenant_id = ?", tenantID).Find(&created)
	for _, item := range created {
		if item.Pid != 0 {
			if nid, ok := idMap[item.Pid]; ok {
				tx.Model(&item).Update("pid", nid)
			}
		}
	}
	return nil
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
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	content = strings.ReplaceAll(content, "{tenantSn}", sn)
	content = strings.ReplaceAll(content, "{tenantId}", util.ToString(tenantID))
	return execSQLScript(content)
}

func execSQLScript(content string) error {
	parts := strings.Split(content, ";\n")
	for _, sql := range parts {
		sql = strings.TrimSpace(sql)
		if sql == "" || strings.HasPrefix(sql, "--") || strings.HasPrefix(sql, "/*") {
			continue
		}
		up := strings.ToUpper(sql)
		if strings.HasPrefix(up, "SET ") || strings.HasPrefix(up, "BEGIN") || strings.HasPrefix(up, "COMMIT") {
			continue
		}
		if err := bootstrap.DB.Exec(sql).Error; err != nil {
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
	content := strings.ReplaceAll(string(raw), "\r\n", "\n")
	content = strings.ReplaceAll(content, "{tenantSn}", sn)
	content = strings.ReplaceAll(content, "`la_", "`"+config.Prefix())
	return execSQLScript(content)
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
		"avatar":    filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"real_name": u.RealName, "nickname": u.Nickname, "account": u.Account, "mobile": u.Mobile,
		"sex": util.SexDesc(u.Sex), "sexCode": u.Sex, "channel": util.ChannelDesc(u.Channel),
		"is_disable": u.IsDisable, "login_ip": u.LoginIP,
		"login_time": util.FormatDateTimePtr(u.LoginTime), "user_money": util.MoneyString(u.UserMoney),
		"create_time": util.FormatDateTime(u.CreateTime),
	}
}
