package platformapi

import (
	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func MenuRoute(c *gin.Context) {
	var admin model.Admin
	if bootstrap.DB.Where("id = ?", mustAdminID(c)).First(&admin).Error != nil {
		response.Data(c, []any{})
		return
	}
	response.Data(c, menuTreeByAdmin(c, admin))
}

func MenuLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	var rows []model.SystemMenu
	bootstrap.DB.Order("sort desc, id asc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, menuMap(m))
	}
	tree := util.LinearToTree(maps, "children", "id", "pid", 0)
	response.Lists(c, tree, int64(len(rows)), q.PageNo, q.PageSize, nil)
}

func MenuAll(c *gin.Context) {
	var rows []model.SystemMenu
	bootstrap.DB.Select("id, pid, name").Where("is_disable = 0").Order("sort desc, id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, map[string]any{"id": m.ID, "pid": m.Pid, "name": m.Name})
	}
	response.Data(c, util.LinearToTree(maps, "children", "id", "pid", 0))
}

func MenuDetail(c *gin.Context) {
	// PHP MenuValidate sceneDetail is id.require; ThinkPHP require treats 0/"0" as present.
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	var m model.SystemMenu
	// PHP MenuLogic::detail is findOrEmpty()->toArray(); unknown/zero id is data: [].
	if bootstrap.DB.First(&m, httpx.QueryUint(c, "id")).Error != nil {
		response.Data(c, []any{})
		return
	}
	response.Data(c, menuMap(m))
}

func MenuAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.MenuWriteCheckTaken(p, false, func(typ, name string) bool {
		return menuUniqueName(0, typ, name) != ""
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !platformMenuParentOK(httpx.BodyUint(c, "pid")) {
		response.Fail(c, "上级菜单不存在")
		return
	}
	m := menuFromReq(c)
	now := util.NowUnix()
	m.CreateTime = now
	m.UpdateTime = util.UnixPtr(now)
	if err := bootstrap.DB.Create(&m).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func MenuEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	id := httpx.BodyUint(c, "id")
	if msg := util.MenuWriteCheckTaken(p, true, func(typ, name string) bool {
		return menuUniqueName(id, typ, name) != ""
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	if id == httpx.BodyUint(c, "pid") {
		response.Fail(c, "上级菜单不能选择自己")
		return
	}
	if !platformMenuParentOK(httpx.BodyUint(c, "pid")) {
		response.Fail(c, "上级菜单不存在")
		return
	}
	// PHP MenuLogic::edit updates by id with no existence check.
	m := menuFromReq(c)
	now := util.NowUnix()
	m.UpdateTime = &now
	if err := bootstrap.DB.Model(&model.SystemMenu{}).Where("id = ?", id).Updates(menuUpdate(m)).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func MenuDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var child int64
	bootstrap.DB.Model(&model.SystemMenu{}).Where("pid = ?", id).Count(&child)
	if child > 0 {
		response.Fail(c, "存在子菜单,不允许删除")
		return
	}
	var bind int64
	bootstrap.DB.Model(&model.SystemRoleMenu{}).Where("menu_id = ?", id).Count(&bind)
	if bind > 0 {
		response.Fail(c, "已分配菜单不可删除")
		return
	}
	bootstrap.DB.Delete(&model.SystemMenu{}, id)
	bootstrap.DB.Where("menu_id = ?", id).Delete(&model.SystemRoleMenu{})
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func MenuUpdateStatus(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	if !httpx.BodyHas(c, "is_disable") {
		response.Fail(c, "请选择菜单状态")
		return
	}
	if !util.InZeroOne(httpx.BodyAny(c, "is_disable")) {
		response.Fail(c, "菜单状态参数值错误")
		return
	}
	id := httpx.BodyUint(c, "id")
	// PHP MenuLogic::updateStatus updates by id with no existence check.
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemMenu{}).Where("id = ?", id).Updates(map[string]any{
		"is_disable":  httpx.BodyInt(c, "is_disable"),
		"update_time": now,
	})
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "操作成功")
}

func RoleLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := bootstrap.DB.Model(&model.SystemRole{}).Where("delete_time IS NULL")
	var count int64
	db.Count(&count)
	var rows []model.SystemRole
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		var num int64
		bootstrap.DB.Model(&model.AdminRole{}).Where("role_id = ?", r.ID).Count(&num)
		var menuIDs []uint
		bootstrap.DB.Model(&model.SystemRoleMenu{}).Where("role_id = ?", r.ID).Pluck("menu_id", &menuIDs)
		if menuIDs == nil {
			menuIDs = []uint{}
		}
		out = append(out, map[string]any{
			"id":          r.ID,
			"name":        r.Name,
			"desc":        r.Desc,
			"sort":        r.Sort,
			"create_time": util.FormatDateTime(r.CreateTime),
			"num":         num,
			"menu_id":     menuIDs,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func RoleAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.RoleWriteCheckTaken(p, false, func(name string) bool {
		return roleNameTaken(0, name)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	menuIDs := httpx.BodyUints(c, "menu_id")
	if !platformMenuIDsOwned(menuIDs) {
		response.Fail(c, "菜单不存在")
		return
	}
	now := util.NowUnix()
	r := model.SystemRole{Name: httpx.BodyStr(c, "name"), Desc: httpx.BodyStr(c, "desc"), Sort: httpx.BodyInt(c, "sort"), CreateTime: now, UpdateTime: util.UnixPtr(now)}
	if err := bootstrap.DB.Create(&r).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	for _, id := range menuIDs {
		bootstrap.DB.Create(&model.SystemRoleMenu{RoleID: r.ID, MenuID: id})
	}
	response.SuccessNotice(c, "添加成功")
}

func RoleEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !httpx.BodyHas(c, "id") || httpx.BodyStr(c, "id") == "" {
		response.Fail(c, "请选择角色")
		return
	}
	id := httpx.BodyUint(c, "id")
	var exist model.SystemRole
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&exist).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	if msg := util.RoleWriteCheckTaken(p, true, func(name string) bool {
		return roleNameTaken(id, name)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemRole{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.BodyStr(c, "name"), "desc": httpx.BodyStr(c, "desc"), "sort": httpx.BodyInt(c, "sort"), "update_time": now,
	})
	if menuIDs := httpx.BodyUints(c, "menu_id"); len(menuIDs) > 0 {
		if !platformMenuIDsOwned(menuIDs) {
			response.Fail(c, "菜单不存在")
			return
		}
		bootstrap.DB.Where("role_id = ?", id).Delete(&model.SystemRoleMenu{})
		for _, mid := range menuIDs {
			bootstrap.DB.Create(&model.SystemRoleMenu{RoleID: id, MenuID: mid})
		}
	}
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "编辑成功")
}

func RoleDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "请选择角色")
		return
	}
	id := httpx.BodyUint(c, "id")
	var exist model.SystemRole
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&exist).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	var used int64
	bootstrap.DB.Model(&model.AdminRole{}).Where("role_id = ?", id).Count(&used)
	if used > 0 {
		response.Fail(c, "有管理员在使用该角色，不允许删除")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemRole{}).Where("id = ?", id).Updates(util.SoftDeleteFields(now))
	bootstrap.DB.Where("role_id = ?", id).Delete(&model.SystemRoleMenu{})
	cache.ClearAdminAuthCache(0)
	response.SuccessNotice(c, "删除成功")
}

func RoleDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "请选择角色")
		return
	}
	var r model.SystemRole
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")).First(&r).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	var menuIDs []uint
	bootstrap.DB.Model(&model.SystemRoleMenu{}).Where("role_id = ?", r.ID).Pluck("menu_id", &menuIDs)
	if menuIDs == nil {
		menuIDs = []uint{}
	}
	response.Data(c, gin.H{"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort, "menu_id": menuIDs})
}

func RoleAll(c *gin.Context) {
	if _, ok := httpx.Query(c)["tenant_id"]; ok {
		tid := httpx.QueryUint(c, "tenant_id")
		if tid == 0 {
			response.Data(c, []any{})
			return
		}
		db := tenantdb.ForTenant(tid)
		if db == nil {
			db = bootstrap.DB
		}
		var rows []model.TenantSystemRole
		q := db.Where("delete_time IS NULL").Where("tenant_id = ?", tid)
		q.Order("sort desc, id desc").Find(&rows)
		out := make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			out = append(out, map[string]any{
				"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort,
				"create_time": util.FormatDateTime(r.CreateTime),
			})
		}
		response.Data(c, out)
		return
	}
	var rows []model.SystemRole
	bootstrap.DB.Where("delete_time IS NULL").Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort,
			"create_time": util.FormatDateTime(r.CreateTime),
		})
	}
	response.Data(c, out)
}

func menuFromReq(c *gin.Context) model.SystemMenu {
	return model.SystemMenu{
		Pid:       httpx.BodyUint(c, "pid"),
		Type:      httpx.BodyStr(c, "type"),
		Name:      httpx.BodyStr(c, "name"),
		Icon:      httpx.BodyStr(c, "icon"),
		Sort:      httpx.BodyInt(c, "sort"),
		Perms:     httpx.BodyStr(c, "perms"),
		Paths:     httpx.BodyStr(c, "paths"),
		Component: httpx.BodyStr(c, "component"),
		Selected:  httpx.BodyStr(c, "selected"),
		Params:    httpx.BodyStr(c, "params"),
		IsCache:   httpx.BodyInt(c, "is_cache"),
		IsShow:    httpx.BodyInt(c, "is_show"),
		IsDisable: httpx.BodyInt(c, "is_disable"),
	}
}

func menuUpdate(m model.SystemMenu) map[string]any {
	return map[string]any{
		"pid": m.Pid, "type": m.Type, "name": m.Name, "icon": m.Icon, "sort": m.Sort,
		"perms": m.Perms, "paths": m.Paths, "component": m.Component, "selected": m.Selected,
		"params": m.Params, "is_cache": m.IsCache, "is_show": m.IsShow, "is_disable": m.IsDisable,
		"update_time": m.UpdateTime,
	}
}

func menuMap(m model.SystemMenu) map[string]any {
	return map[string]any{
		"id": m.ID, "pid": m.Pid, "type": m.Type, "name": m.Name, "icon": m.Icon, "sort": m.Sort,
		"perms": m.Perms, "paths": m.Paths, "component": m.Component, "selected": m.Selected,
		"params": m.Params, "is_cache": m.IsCache, "is_show": m.IsShow, "is_disable": m.IsDisable,
		"create_time": util.FormatDateTime(m.CreateTime),
		"update_time": util.FormatDateTimePtr(m.UpdateTime),
	}
}

func menuTreeByAdmin(c *gin.Context, admin model.Admin) []map[string]any {
	var rows []model.SystemMenu
	q := bootstrap.DB.Where("is_disable = 0 AND type IN ?", []string{"M", "C"})
	if admin.Root != 1 {
		roleIDs, _, _ := adminRelations(admin.ID)
		var menuIDs []uint
		if len(roleIDs) > 0 {
			bootstrap.DB.Model(&model.SystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
		}
		if len(menuIDs) == 0 {
			return []map[string]any{}
		}
		q = q.Where("id IN ?", menuIDs)
	}
	q.Order("sort desc, id asc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, menuMap(m))
	}
	return util.LinearToTree(maps, "children", "id", "pid", 0)
}

func buttonPerms(admin model.Admin) []string {
	if admin.Root == 1 {
		return []string{"*"}
	}
	roleIDs, _, _ := adminRelations(admin.ID)
	var menuIDs []uint
	if len(roleIDs) > 0 {
		bootstrap.DB.Model(&model.SystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
	}
	return authsvc.BtnAuth(false, platformMenuPerms(menuIDs, true), platformMenuPerms(nil, false))
}

func platformMenuPerms(menuIDs []uint, filterIDs bool) []string {
	q := bootstrap.DB.Model(&model.SystemMenu{}).Where("is_disable = 0 AND perms <> ''")
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

func mustAdminID(c *gin.Context) uint {
	return ctxutil.Get(c).AdminID
}

func platformMenuParentOK(pid uint) bool {
	if pid == 0 {
		return true
	}
	var parent model.SystemMenu
	return bootstrap.DB.Where("id = ?", pid).First(&parent).Error == nil
}

func menuUniqueName(id uint, typ, name string) string {
	if typ != "M" {
		return ""
	}
	var n int64
	q := bootstrap.DB.Model(&model.SystemMenu{}).Where("type = ? AND name = ?", typ, name)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	if n > 0 {
		return "菜单名称已存在"
	}
	return ""
}

func platformMenuIDsOwned(ids []uint) bool {
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
	var n int64
	bootstrap.DB.Model(&model.SystemMenu{}).Where("id IN ?", uniq).Count(&n)
	return n == int64(len(uniq))
}

func roleNameTaken(id uint, name string) bool {
	var n int64
	q := bootstrap.DB.Model(&model.SystemRole{}).Where("name = ? AND delete_time IS NULL", name)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}
