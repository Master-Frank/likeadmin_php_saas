package platformapi

import (
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
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
	q := lists.Parse(c)
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
	bootstrap.DB.Select("id, pid, name").Order("sort desc, id asc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, map[string]any{"id": m.ID, "pid": m.Pid, "name": m.Name})
	}
	response.Data(c, util.LinearToTree(maps, "children", "id", "pid", 0))
}

func MenuDetail(c *gin.Context) {
	var m model.SystemMenu
	if bootstrap.DB.First(&m, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "菜单不存在")
		return
	}
	response.Data(c, menuMap(m))
}

func MenuAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.MenuWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if msg := menuUniqueName(0, httpx.Str(c, "type"), httpx.Str(c, "name")); msg != "" {
		response.Fail(c, msg)
		return
	}
	m := menuFromReq(c)
	m.CreateTime = util.NowUnix()
	if err := bootstrap.DB.Create(&m).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
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
	if msg := menuUniqueName(id, httpx.Str(c, "type"), httpx.Str(c, "name")); msg != "" {
		response.Fail(c, msg)
		return
	}
	m := menuFromReq(c)
	now := util.NowUnix()
	m.UpdateTime = &now
	if err := bootstrap.DB.Model(&model.SystemMenu{}).Where("id = ?", id).Updates(menuUpdate(m)).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func MenuDelete(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.Uint(c, "id")
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
	response.SuccessNotice(c, "操作成功")
}

func MenuUpdateStatus(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemMenu{}).Where("id = ?", id).Updates(map[string]any{
		"is_disable":  httpx.Int(c, "is_disable"),
		"update_time": now,
	})
	response.SuccessNotice(c, "操作成功")
}

func RoleLists(c *gin.Context) {
	q := lists.Parse(c)
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
	p := httpx.Params(c)
	if msg := util.RoleWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if roleNameTaken(0, httpx.Str(c, "name")) {
		response.Fail(c, "角色名称已存在")
		return
	}
	now := util.NowUnix()
	r := model.SystemRole{Name: httpx.Str(c, "name"), Desc: httpx.Str(c, "desc"), Sort: httpx.Int(c, "sort"), CreateTime: now}
	if err := bootstrap.DB.Create(&r).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	for _, id := range httpx.Uints(c, "menu_id") {
		bootstrap.DB.Create(&model.SystemRoleMenu{RoleID: r.ID, MenuID: id})
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
	var exist model.SystemRole
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&exist).Error != nil {
		response.Fail(c, "角色不存在")
		return
	}
	if roleNameTaken(id, httpx.Str(c, "name")) {
		response.Fail(c, "角色名称已存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemRole{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "desc": httpx.Str(c, "desc"), "sort": httpx.Int(c, "sort"), "update_time": now,
	})
	bootstrap.DB.Where("role_id = ?", id).Delete(&model.SystemRoleMenu{})
	for _, mid := range httpx.Uints(c, "menu_id") {
		bootstrap.DB.Create(&model.SystemRoleMenu{RoleID: id, MenuID: mid})
	}
	response.SuccessNotice(c, "编辑成功")
}

func RoleDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "请选择角色")
		return
	}
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
	bootstrap.DB.Model(&model.SystemRole{}).Where("id = ?", id).Update("delete_time", now)
	bootstrap.DB.Where("role_id = ?", id).Delete(&model.SystemRoleMenu{})
	response.SuccessNotice(c, "删除成功")
}

func RoleDetail(c *gin.Context) {
	var r model.SystemRole
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&r).Error != nil {
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
		Pid:       httpx.Uint(c, "pid"),
		Type:      httpx.Str(c, "type"),
		Name:      httpx.Str(c, "name"),
		Icon:      httpx.Str(c, "icon"),
		Sort:      httpx.Int(c, "sort"),
		Perms:     httpx.Str(c, "perms"),
		Paths:     httpx.Str(c, "paths"),
		Component: httpx.Str(c, "component"),
		Selected:  httpx.Str(c, "selected"),
		Params:    httpx.Str(c, "params"),
		IsCache:   httpx.Int(c, "is_cache"),
		IsShow:    httpx.Int(c, "is_show"),
		IsDisable: httpx.Int(c, "is_disable"),
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
	var menus []model.SystemMenu
	if len(menuIDs) > 0 {
		bootstrap.DB.Where("id IN ? AND perms <> ''", menuIDs).Find(&menus)
	}
	out := []string{}
	for _, m := range menus {
		if m.Perms != "" {
			out = append(out, m.Perms)
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func mustAdminID(c *gin.Context) uint {
	return ctxutil.Get(c).AdminID
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

func roleNameTaken(id uint, name string) bool {
	var n int64
	q := bootstrap.DB.Model(&model.SystemRole{}).Where("name = ? AND delete_time IS NULL", name)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}
