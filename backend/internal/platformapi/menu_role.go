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
	m := menuFromReq(c)
	m.CreateTime = util.NowUnix()
	if err := bootstrap.DB.Create(&m).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "添加成功", nil)
}

func MenuEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	m := menuFromReq(c)
	now := util.NowUnix()
	m.UpdateTime = &now
	if err := bootstrap.DB.Model(&model.SystemMenu{}).Where("id = ?", id).Updates(menuUpdate(m)).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "修改成功", nil)
}

func MenuDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var child int64
	bootstrap.DB.Model(&model.SystemMenu{}).Where("pid = ?", id).Count(&child)
	if child > 0 {
		response.Fail(c, "请先删除子菜单")
		return
	}
	bootstrap.DB.Delete(&model.SystemMenu{}, id)
	bootstrap.DB.Where("menu_id = ?", id).Delete(&model.SystemRoleMenu{})
	response.Success(c, "删除成功", nil)
}

func MenuUpdateStatus(c *gin.Context) {
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemMenu{}).Where("id = ?", id).Updates(map[string]any{
		"is_disable":  httpx.Int(c, "is_disable"),
		"update_time": now,
	})
	response.Success(c, "修改成功", nil)
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
	now := util.NowUnix()
	r := model.SystemRole{Name: httpx.Str(c, "name"), Desc: httpx.Str(c, "desc"), Sort: httpx.Int(c, "sort"), CreateTime: now}
	if err := bootstrap.DB.Create(&r).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	for _, id := range httpx.Uints(c, "menu_id") {
		bootstrap.DB.Create(&model.SystemRoleMenu{RoleID: r.ID, MenuID: id})
	}
	response.Success(c, "添加成功", nil)
}

func RoleEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemRole{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "desc": httpx.Str(c, "desc"), "sort": httpx.Int(c, "sort"), "update_time": now,
	})
	bootstrap.DB.Where("role_id = ?", id).Delete(&model.SystemRoleMenu{})
	for _, mid := range httpx.Uints(c, "menu_id") {
		bootstrap.DB.Create(&model.SystemRoleMenu{RoleID: id, MenuID: mid})
	}
	response.Success(c, "修改成功", nil)
}

func RoleDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	bootstrap.DB.Model(&model.SystemRole{}).Where("id = ?", id).Update("delete_time", now)
	bootstrap.DB.Where("role_id = ?", id).Delete(&model.SystemRoleMenu{})
	response.Success(c, "删除成功", nil)
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
	response.Data(c, rows)
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
