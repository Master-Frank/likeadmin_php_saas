package tenantapi

import (
	"strings"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AdminLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.TenantAdmin{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if account := lists.Param(q, "account"); account != "" {
		db = db.Where("account LIKE ?", "%"+account+"%")
	}
	if rid := lists.Param(q, "role_id"); rid != "" {
		var ids []uint
		tdb(c).Model(&model.TenantAdminRole{}).Where("role_id = ?", lists.ParamInt(q, "role_id")).Pluck("admin_id", &ids)
		if len(ids) > 0 {
			db = db.Where("id IN ?", ids)
		}
	}
	var count int64
	db.Count(&count)
	order := lists.OrderSQL(q, "id desc", map[string]bool{"create_time": true, "id": true})
	var rows []model.TenantAdmin
	db.Order(order).Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, tenantAdminListItem(c, a))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func tenantAdminListItem(c *gin.Context, a model.TenantAdmin) map[string]any {
	roleIDs, deptIDs, jobIDs := tenantAdminRelations(c, a.ID)
	roleName := joinNames(tenantRoleNames(c, roleIDs))
	if a.Root == 1 {
		roleName = "系统管理员"
	}
	disableDesc := "正常"
	if a.Disable == 1 {
		disableDesc = "禁用"
	}
	return map[string]any{
		"id":               a.ID,
		"name":             a.Name,
		"account":          a.Account,
		"create_time":      util.FormatDateTime(a.CreateTime),
		"disable":          a.Disable,
		"root":             a.Root,
		"login_time":       util.FormatDateTimePtr(a.LoginTime),
		"login_ip":         a.LoginIP,
		"multipoint_login": a.MultipointLogin,
		"avatar":           filesvc.GetFileURL(c, firstNonEmpty(a.Avatar, config.C.Project.Tenant["admin_avatar"])),
		"role_id":          roleIDs,
		"dept_id":          deptIDs,
		"jobs_id":          jobIDs,
		"disable_desc":     disableDesc,
		"role_name":        roleName,
		"dept_name":        joinNames(tenantDeptNames(c, deptIDs)),
		"jobs_name":        joinNames(tenantJobNames(c, jobIDs)),
	}
}

func AdminAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.AuthAdminAddCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	account := httpx.Str(c, "account")
	name := httpx.Str(c, "name")
	if tenantAdminAccountTaken(c, account, 0) {
		response.Fail(c, "账号已存在")
		return
	}
	if tenantAdminNameTaken(c, name, 0) {
		response.Fail(c, "名称已存在")
		return
	}
	avatar := filesvc.SetFileURL(c, httpx.Str(c, "avatar"))
	if avatar == "" {
		avatar = config.C.Project.DefaultImage["admin_avatar"]
	}
	disable := 0
	if _, ok := p["disable"]; ok {
		disable = httpx.Int(c, "disable")
	}
	admin := model.TenantAdmin{
		TenantID: tenantDB(c), Name: name, Account: account,
		Password: util.CreatePassword(httpx.Str(c, "password"), config.C.Project.UniqueIdentification),
		Disable:  disable, MultipointLogin: httpx.Int(c, "multipoint_login"),
		Avatar: avatar, CreateTime: util.NowUnix(),
	}
	roles, depts, jobs := httpx.Uints(c, "role_id"), httpx.Uints(c, "dept_id"), httpx.Uints(c, "jobs_id")
	if msg := tenantAuthLinksCheck(c, roles, depts, jobs); msg != "" {
		response.Fail(c, msg)
		return
	}
	err := tdb(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		return saveTenantAuthLinks(tx, admin.ID, roles, depts, jobs)
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func AdminEdit(c *gin.Context) {
	p := httpx.Params(c)
	if !authAdminIDPresent(p) {
		response.Fail(c, "管理员id不能为空")
		return
	}
	id := httpx.Uint(c, "id")
	admin, ok := tenantAdminByID(c, id)
	if !ok {
		response.Fail(c, "管理员不存在")
		return
	}
	if msg := util.AuthAdminEditCheck(p, admin.Root == 1); msg != "" {
		response.Fail(c, msg)
		return
	}
	account := httpx.Str(c, "account")
	name := httpx.Str(c, "name")
	if tenantAdminAccountTaken(c, account, id) {
		response.Fail(c, "账号已存在")
		return
	}
	if tenantAdminNameTaken(c, name, id) {
		response.Fail(c, "名称已存在")
		return
	}
	avatar := ""
	if v := httpx.Str(c, "avatar"); v != "" {
		avatar = filesvc.SetFileURL(c, v)
	}
	data := map[string]any{
		"name":             name,
		"account":          account,
		"disable":          httpx.Int(c, "disable"),
		"multipoint_login": httpx.Int(c, "multipoint_login"),
		"avatar":           avatar,
		"update_time":      util.NowUnix(),
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	var oldRoles []uint
	tdb(c).Model(&model.TenantAdminRole{}).Where("admin_id = ?", id).Pluck("role_id", &oldRoles)
	newRoles := httpx.Uints(c, "role_id")
	depts, jobs := httpx.Uints(c, "dept_id"), httpx.Uints(c, "jobs_id")
	if msg := tenantAuthLinksCheck(c, newRoles, depts, jobs); msg != "" {
		response.Fail(c, msg)
		return
	}
	err := tdb(c).Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&model.TenantAdmin{}).Where("id = ?", id)
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		if err := q.Updates(data).Error; err != nil {
			return err
		}
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminRole{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminDept{})
		tx.Where("admin_id = ?", id).Delete(&model.TenantAdminJobs{})
		return saveTenantAuthLinks(tx, id, newRoles, depts, jobs)
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	if httpx.Int(c, "disable") == 1 || util.UintSlicesChanged(oldRoles, newRoles) {
		expireTenantAuthTokens(c, id)
	}
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func AdminEditSelf(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.AdminEditSelfCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := ctxutil.Get(c).AdminID
	admin, ok := tenantAdminByID(c, id)
	if !ok {
		response.Fail(c, "管理员不存在")
		return
	}
	data := map[string]any{
		"name": httpx.Str(c, "name"), "avatar": filesvc.SetFileURL(c, httpx.Str(c, "avatar")), "update_time": util.NowUnix(),
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		old := httpx.Str(c, "password_old")
		if admin.Password != util.CreatePassword(old, config.C.Project.UniqueIdentification) {
			response.Fail(c, "当前密码错误")
			return
		}
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	tdb(c).Model(&admin).Updates(data)
	if pwd := httpx.Str(c, "password"); pwd != "" {
		expireTenantAuthTokens(c, id)
	}
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func AdminDelete(c *gin.Context) {
	p := httpx.Params(c)
	if !authAdminIDPresent(p) {
		response.Fail(c, "管理员id不能为空")
		return
	}
	id := httpx.Uint(c, "id")
	a, ok := tenantAdminByID(c, id)
	if !ok {
		response.Fail(c, "管理员不存在")
		return
	}
	if a.Root == 1 {
		response.Fail(c, "超级管理员不允许被删除")
		return
	}
	err := tdb(c).Transaction(func(tx *gorm.DB) error {
		now := util.NowUnix()
		q := tx.Model(&model.TenantAdmin{}).Where("id = ?", id)
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		if err := q.Update("delete_time", now).Error; err != nil {
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
	expireTenantAuthTokens(c, id)
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func AdminDetail(c *gin.Context) {
	p := httpx.Params(c)
	if !authAdminIDPresent(p) {
		response.Fail(c, "管理员id不能为空")
		return
	}
	id := httpx.Uint(c, "id")
	a, ok := tenantAdminByID(c, id)
	if !ok {
		response.Fail(c, "管理员不存在")
		return
	}
	roleIDs, deptIDs, jobIDs := tenantAdminRelations(c, a.ID)
	response.Data(c, gin.H{
		"id": a.ID, "account": a.Account, "name": a.Name, "disable": a.Disable, "root": a.Root,
		"multipoint_login": a.MultipointLogin,
		"avatar":           filesvc.GetFileURL(c, firstNonEmpty(a.Avatar, config.C.Project.Tenant["admin_avatar"])),
		"role_id":          roleIDs, "dept_id": deptIDs, "jobs_id": jobIDs,
	})
}

func MenuRoute(c *gin.Context) {
	var admin model.TenantAdmin
	if scopeTID(tdb(c).Where("id = ?", ctxutil.Get(c).AdminID), c).First(&admin).Error != nil {
		response.Data(c, []any{})
		return
	}
	response.Data(c, tenantMenuTreeByAdmin(c, admin))
}

func MenuLists(c *gin.Context) {
	q := lists.Parse(c)
	var rows []model.TenantSystemMenu
	db := tdb(c).Order("sort desc, id asc")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, tenantMenuMap(m))
	}
	response.Lists(c, util.LinearToTree(maps, "children", "id", "pid", 0), int64(len(rows)), q.PageNo, q.PageSize, nil)
}

func MenuAll(c *gin.Context) {
	var rows []model.TenantSystemMenu
	db := tdb(c).Select("id,pid,name").Where("is_disable = 0")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, map[string]any{"id": m.ID, "pid": m.Pid, "name": m.Name})
	}
	response.Data(c, util.LinearToTree(maps, "children", "id", "pid", 0))
}

func MenuAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.MenuWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if msg := tenantMenuUniqueName(c, 0, httpx.Str(c, "type"), httpx.Str(c, "name")); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !tenantMenuParentOK(c, httpx.Uint(c, "pid")) {
		response.Fail(c, "上级菜单不存在")
		return
	}
	m := tenantMenuFromReq(c)
	m.TenantID = tenantDB(c)
	m.CreateTime = util.NowUnix()
	tdb(c).Create(&m)
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func MenuEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.MenuWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	if id == httpx.Uint(c, "pid") {
		response.Fail(c, "上级菜单不能选择自己")
		return
	}
	if !tenantMenuParentOK(c, httpx.Uint(c, "pid")) {
		response.Fail(c, "上级菜单不存在")
		return
	}
	if msg := tenantMenuUniqueName(c, id, httpx.Str(c, "type"), httpx.Str(c, "name")); msg != "" {
		response.Fail(c, msg)
		return
	}
	var exist model.TenantSystemMenu
	if scopeTID(tdb(c).Where("id = ?", id), c).First(&exist).Error != nil {
		response.Fail(c, "菜单不存在")
		return
	}
	m := tenantMenuFromReq(c)
	now := util.NowUnix()
	scopeTID(tdb(c).Model(&model.TenantSystemMenu{}).Where("id = ?", id), c).Updates(map[string]any{
		"pid": m.Pid, "type": m.Type, "name": m.Name, "icon": m.Icon, "sort": m.Sort, "perms": m.Perms,
		"paths": m.Paths, "component": m.Component, "selected": m.Selected, "params": m.Params,
		"is_cache": m.IsCache, "is_show": m.IsShow, "is_disable": m.IsDisable, "update_time": now,
	})
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func MenuDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var child int64
	scopeTID(tdb(c).Model(&model.TenantSystemMenu{}).Where("pid = ?", id), c).Count(&child)
	if child > 0 {
		response.Fail(c, "存在子菜单,不允许删除")
		return
	}
	var bind int64
	tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("menu_id = ?", id).Count(&bind)
	if bind > 0 {
		response.Fail(c, "已分配菜单不可删除")
		return
	}
	scopeTID(tdb(c).Where("id = ?", id), c).Delete(&model.TenantSystemMenu{})
	tdb(c).Where("menu_id = ?", id).Delete(&model.TenantSystemRoleMenu{})
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func MenuDetail(c *gin.Context) {
	var m model.TenantSystemMenu
	scopeTID(tdb(c).Where("id = ?", httpx.Uint(c, "id")), c).First(&m)
	response.Data(c, tenantMenuMap(m))
}

func MenuUpdateStatus(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var exist model.TenantSystemMenu
	if scopeTID(tdb(c).Where("id = ?", httpx.Uint(c, "id")), c).First(&exist).Error != nil {
		response.Fail(c, "菜单不存在")
		return
	}
	scopeTID(tdb(c).Model(&model.TenantSystemMenu{}).Where("id = ?", httpx.Uint(c, "id")), c).Update("is_disable", httpx.Int(c, "is_disable"))
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func RoleLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.TenantSystemRole{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var count int64
	db.Count(&count)
	var rows []model.TenantSystemRole
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		var menuIDs []uint
		tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("role_id = ?", r.ID).Pluck("menu_id", &menuIDs)
		var num int64
		tdb(c).Model(&model.TenantAdminRole{}).Where("role_id = ?", r.ID).Count(&num)
		if menuIDs == nil {
			menuIDs = []uint{}
		}
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort,
			"create_time": util.FormatDateTime(r.CreateTime), "num": num, "menu_id": menuIDs,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func RoleAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.RoleWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if tenantRoleNameTaken(c, 0, httpx.Str(c, "name")) {
		response.Fail(c, "角色名称已存在")
		return
	}
	menuIDs := httpx.Uints(c, "menu_id")
	if !tenantIDsOwned(c, &model.TenantSystemMenu{}, menuIDs, "") {
		response.Fail(c, "菜单不存在")
		return
	}
	r := model.TenantSystemRole{Name: httpx.Str(c, "name"), Desc: httpx.Str(c, "desc"), Sort: httpx.Int(c, "sort"), TenantID: tenantDB(c), CreateTime: util.NowUnix()}
	tdb(c).Create(&r)
	for _, id := range menuIDs {
		tdb(c).Create(&model.TenantSystemRoleMenu{RoleID: r.ID, MenuID: id})
	}
	response.SuccessNotice(c, "添加成功")
}

func RoleEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.RoleWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	var exist model.TenantSystemRole
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&exist).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	if tenantRoleNameTaken(c, id, httpx.Str(c, "name")) {
		response.Fail(c, "角色名称已存在")
		return
	}
	scopeTID(tdb(c).Model(&model.TenantSystemRole{}).Where("id = ?", id), c).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "desc": httpx.Str(c, "desc"), "sort": httpx.Int(c, "sort"),
	})
	if menuIDs := httpx.Uints(c, "menu_id"); len(menuIDs) > 0 {
		if !tenantIDsOwned(c, &model.TenantSystemMenu{}, menuIDs, "") {
			response.Fail(c, "菜单不存在")
			return
		}
		tdb(c).Where("role_id = ?", id).Delete(&model.TenantSystemRoleMenu{})
		for _, mid := range menuIDs {
			tdb(c).Create(&model.TenantSystemRoleMenu{RoleID: id, MenuID: mid})
		}
	}
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "编辑成功")
}

func RoleDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "请选择角色")
		return
	}
	var exist model.TenantSystemRole
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&exist).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	var used int64
	tdb(c).Model(&model.TenantAdminRole{}).Where("role_id = ?", id).Count(&used)
	if used > 0 {
		response.Fail(c, "有管理员在使用该角色，不允许删除")
		return
	}
	now := util.NowUnix()
	scopeTID(tdb(c).Model(&model.TenantSystemRole{}).Where("id = ?", id), c).Update("delete_time", now)
	tdb(c).Where("role_id = ?", id).Delete(&model.TenantSystemRoleMenu{})
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "删除成功")
}

func RoleDetail(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "请选择角色")
		return
	}
	var r model.TenantSystemRole
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")), c).First(&r).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	var menuIDs []uint
	tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("role_id = ?", r.ID).Pluck("menu_id", &menuIDs)
	if menuIDs == nil {
		menuIDs = []uint{}
	}
	response.Data(c, gin.H{
		"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort, "menu_id": menuIDs,
	})
}

func RoleAll(c *gin.Context) {
	var rows []model.TenantSystemRole
	db := tdb(c).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort,
			"create_time": util.FormatDateTime(r.CreateTime),
		})
	}
	response.Data(c, out)
}

func authAdminIDPresent(p map[string]any) bool {
	v, ok := p["id"]
	if !ok || v == nil {
		return false
	}
	return strings.TrimSpace(util.ToString(v)) != ""
}

func tenantAdminByID(c *gin.Context, id uint) (model.TenantAdmin, bool) {
	var admin model.TenantAdmin
	if id == 0 {
		return admin, false
	}
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&admin).Error != nil || admin.ID == 0 {
		return admin, false
	}
	return admin, true
}

func tenantAdminAccountTaken(c *gin.Context, account string, excludeID uint) bool {
	q := scopeTID(tdb(c).Model(&model.TenantAdmin{}).Where("account = ? AND delete_time IS NULL", account), c)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	q.Count(&n)
	return n > 0
}

func tenantAdminNameTaken(c *gin.Context, name string, excludeID uint) bool {
	q := scopeTID(tdb(c).Model(&model.TenantAdmin{}).Where("name = ? AND delete_time IS NULL", name), c)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	var n int64
	q.Count(&n)
	return n > 0
}

func tenantAdminRelations(c *gin.Context, id uint) (roles, depts, jobs []uint) {
	tdb(c).Model(&model.TenantAdminRole{}).Where("admin_id = ?", id).Pluck("role_id", &roles)
	tdb(c).Model(&model.TenantAdminDept{}).Where("admin_id = ?", id).Pluck("dept_id", &depts)
	tdb(c).Model(&model.TenantAdminJobs{}).Where("admin_id = ?", id).Pluck("jobs_id", &jobs)
	if roles == nil {
		roles = []uint{}
	}
	if depts == nil {
		depts = []uint{}
	}
	if jobs == nil {
		jobs = []uint{}
	}
	return
}

func tenantAuthLinksCheck(c *gin.Context, roles, depts, jobs []uint) string {
	if !tenantIDsOwned(c, &model.TenantSystemRole{}, roles, "delete_time IS NULL") {
		return "角色不存在"
	}
	if !tenantIDsOwned(c, &model.TenantDept{}, depts, "delete_time IS NULL") {
		return "部门不存在"
	}
	if !tenantIDsOwned(c, &model.TenantJobs{}, jobs, "delete_time IS NULL") {
		return "岗位不存在"
	}
	return ""
}

func tenantIDsOwned(c *gin.Context, dest any, ids []uint, extra string) bool {
	uniq := uniquePositiveUints(ids)
	if len(uniq) == 0 {
		return true
	}
	q := scopeTID(tdb(c).Model(dest).Where("id IN ?", uniq), c)
	if extra != "" {
		q = q.Where(extra)
	}
	var n int64
	q.Count(&n)
	return n == int64(len(uniq))
}

func uniquePositiveUints(ids []uint) []uint {
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func saveTenantAuthLinks(tx *gorm.DB, adminID uint, roles, depts, jobs []uint) error {
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

func expireTenantAuthTokens(c *gin.Context, adminID uint) {
	var sess []model.TenantAdminSession
	tdb(c).Where("admin_id = ?", adminID).Find(&sess)
	now := util.NowUnix()
	for _, s := range sess {
		tdb(c).Model(&s).Updates(map[string]any{"expire_time": now, "update_time": now})
		cache.DeleteTenantAdminInfo(s.Token)
	}
}

func tenantRoleNames(c *gin.Context, ids []uint) []string {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.TenantSystemRole
	scopeTID(tdb(c).Where("id IN ?", ids), c).Find(&rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

func tenantDeptNames(c *gin.Context, ids []uint) []string {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.TenantDept
	scopeTID(tdb(c).Where("id IN ?", ids), c).Find(&rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

func tenantJobNames(c *gin.Context, ids []uint) []string {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.TenantJobs
	scopeTID(tdb(c).Where("id IN ?", ids), c).Find(&rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

func joinNames(names []string) string {
	s := ""
	for i, n := range names {
		if i > 0 {
			s += "/"
		}
		s += n
	}
	return s
}

func tenantMenuParentOK(c *gin.Context, pid uint) bool {
	if pid == 0 {
		return true
	}
	var parent model.TenantSystemMenu
	return scopeTID(tdb(c).Where("id = ?", pid), c).First(&parent).Error == nil
}

func tenantMenuUniqueName(c *gin.Context, id uint, typ, name string) string {
	if typ != "M" {
		return ""
	}
	var n int64
	q := scopeTID(tdb(c).Model(&model.TenantSystemMenu{}).Where("type = ? AND name = ?", typ, name), c)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	if n > 0 {
		return "菜单名称已存在"
	}
	return ""
}

func tenantRoleNameTaken(c *gin.Context, id uint, name string) bool {
	var n int64
	q := scopeTID(tdb(c).Model(&model.TenantSystemRole{}).Where("name = ? AND delete_time IS NULL", name), c)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func tenantMenuFromReq(c *gin.Context) model.TenantSystemMenu {
	return model.TenantSystemMenu{
		Pid: httpx.Uint(c, "pid"), Type: httpx.Str(c, "type"), Name: httpx.Str(c, "name"),
		Icon: httpx.Str(c, "icon"), Sort: httpx.Int(c, "sort"), Perms: httpx.Str(c, "perms"),
		Paths: httpx.Str(c, "paths"), Component: httpx.Str(c, "component"), Selected: httpx.Str(c, "selected"),
		Params: httpx.Str(c, "params"), IsCache: httpx.Int(c, "is_cache"), IsShow: httpx.Int(c, "is_show"),
		IsDisable: httpx.Int(c, "is_disable"),
	}
}

func tenantMenuMap(m model.TenantSystemMenu) map[string]any {
	return map[string]any{
		"id": m.ID, "pid": m.Pid, "type": m.Type, "name": m.Name, "icon": m.Icon, "sort": m.Sort,
		"perms": m.Perms, "paths": m.Paths, "component": m.Component, "selected": m.Selected,
		"params": m.Params, "is_cache": m.IsCache, "is_show": m.IsShow, "is_disable": m.IsDisable,
		"tenant_id": m.TenantID, "create_time": util.FormatDateTime(m.CreateTime),
		"update_time": util.FormatDateTimeOrNil(m.UpdateTime),
	}
}

func tenantMenuTreeByAdmin(c *gin.Context, admin model.TenantAdmin) []map[string]any {
	var rows []model.TenantSystemMenu
	q := tdb(c).Where("is_disable = 0 AND type IN ?", []string{"M", "C"})
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if admin.Root != 1 {
		var roleIDs []uint
		tdb(c).Model(&model.TenantAdminRole{}).Where("admin_id = ?", admin.ID).Pluck("role_id", &roleIDs)
		var menuIDs []uint
		if len(roleIDs) > 0 {
			tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
		}
		if len(menuIDs) == 0 {
			return []map[string]any{}
		}
		q = q.Where("id IN ?", menuIDs)
	}
	q.Order("sort desc, id asc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, tenantMenuMap(m))
	}
	return util.LinearToTree(maps, "children", "id", "pid", 0)
}
