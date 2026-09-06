package tenantapi

import (
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
	var count int64
	db.Count(&count)
	var rows []model.TenantAdmin
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		var roleIDs []uint
		tdb(c).Model(&model.TenantAdminRole{}).Where("admin_id = ?", a.ID).Pluck("role_id", &roleIDs)
		out = append(out, map[string]any{
			"id": a.ID, "name": a.Name, "account": a.Account, "root": a.Root, "disable": a.Disable,
			"avatar": filesvc.GetFileURL(c, a.Avatar), "multipoint_login": a.MultipointLogin,
			"create_time": util.FormatDateTime(a.CreateTime), "role_id": roleIDs,
			"login_time": util.FormatDateTimePtr(a.LoginTime), "login_ip": a.LoginIP,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func AdminAdd(c *gin.Context) {
	if msg := util.AdminWriteCheck(httpx.Str(c, "account"), httpx.Str(c, "name"), httpx.Str(c, "password"), true); msg != "" {
		response.Fail(c, msg)
		return
	}
	var exist model.TenantAdmin
	q := tdb(c).Where("account = ? AND delete_time IS NULL", httpx.Str(c, "account"))
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	admin := model.TenantAdmin{
		TenantID: tenantDB(c), Name: httpx.Str(c, "name"), Account: httpx.Str(c, "account"),
		Password: util.CreatePassword(httpx.Str(c, "password"), config.C.Project.UniqueIdentification),
		Disable:  httpx.Int(c, "disable"), MultipointLogin: httpx.Int(c, "multipoint_login"),
		Avatar: filesvc.SetFileURL(c, httpx.Str(c, "avatar")), CreateTime: util.NowUnix(),
	}
	err := tdb(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		for _, id := range httpx.Uints(c, "role_id") {
			if err := tx.Create(&model.TenantAdminRole{AdminID: admin.ID, RoleID: id}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func AdminEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	data := map[string]any{
		"name": httpx.Str(c, "name"), "account": httpx.Str(c, "account"),
		"disable": httpx.Int(c, "disable"), "multipoint_login": httpx.Int(c, "multipoint_login"),
		"avatar": filesvc.SetFileURL(c, httpx.Str(c, "avatar")), "update_time": util.NowUnix(),
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	tdb(c).Model(&model.TenantAdmin{}).Where("id = ?", id).Updates(data)
	tdb(c).Where("admin_id = ?", id).Delete(&model.TenantAdminRole{})
	for _, rid := range httpx.Uints(c, "role_id") {
		tdb(c).Create(&model.TenantAdminRole{AdminID: id, RoleID: rid})
	}
	response.Success(c, "修改成功", nil)
}

func AdminEditSelf(c *gin.Context) {
	id := ctxutil.Get(c).AdminID
	var admin model.TenantAdmin
	if tdb(c).Where("id = ? AND delete_time IS NULL", id).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	data := map[string]any{
		"name": httpx.Str(c, "name"), "avatar": filesvc.SetFileURL(c, httpx.Str(c, "avatar")), "update_time": util.NowUnix(),
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		old := httpx.Str(c, "password_old")
		if old != "" && admin.Password != util.CreatePassword(old, config.C.Project.UniqueIdentification) {
			response.Fail(c, "原密码错误")
			return
		}
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	tdb(c).Model(&admin).Updates(data)
	response.Success(c, "修改成功", nil)
}

func AdminDelete(c *gin.Context) {
	var a model.TenantAdmin
	if tdb(c).First(&a, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	if a.Root == 1 {
		response.Fail(c, "超级管理员不允许被删除")
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&a).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func AdminDetail(c *gin.Context) {
	var a model.TenantAdmin
	if tdb(c).First(&a, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	var roleIDs []uint
	tdb(c).Model(&model.TenantAdminRole{}).Where("admin_id = ?", a.ID).Pluck("role_id", &roleIDs)
	response.Data(c, gin.H{
		"id": a.ID, "name": a.Name, "account": a.Account, "disable": a.Disable, "root": a.Root,
		"multipoint_login": a.MultipointLogin, "avatar": filesvc.GetFileURL(c, a.Avatar), "role_id": roleIDs,
	})
}

func MenuRoute(c *gin.Context) {
	AdminMySelf(c)
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
	db := tdb(c).Select("id,pid,name")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, m := range rows {
		maps = append(maps, map[string]any{"id": m.ID, "pid": m.Pid, "name": m.Name})
	}
	response.Data(c, util.LinearToTree(maps, "children", "id", "pid", 0))
}

func MenuAdd(c *gin.Context) {
	m := tenantMenuFromReq(c)
	m.TenantID = tenantDB(c)
	m.CreateTime = util.NowUnix()
	tdb(c).Create(&m)
	response.Success(c, "添加成功", nil)
}

func MenuEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	m := tenantMenuFromReq(c)
	now := util.NowUnix()
	tdb(c).Model(&model.TenantSystemMenu{}).Where("id = ?", id).Updates(map[string]any{
		"pid": m.Pid, "type": m.Type, "name": m.Name, "icon": m.Icon, "sort": m.Sort, "perms": m.Perms,
		"paths": m.Paths, "component": m.Component, "selected": m.Selected, "params": m.Params,
		"is_cache": m.IsCache, "is_show": m.IsShow, "is_disable": m.IsDisable, "update_time": now,
	})
	response.Success(c, "修改成功", nil)
}

func MenuDelete(c *gin.Context) {
	tdb(c).Delete(&model.TenantSystemMenu{}, httpx.Uint(c, "id"))
	response.Success(c, "删除成功", nil)
}

func MenuDetail(c *gin.Context) {
	var m model.TenantSystemMenu
	tdb(c).First(&m, httpx.Uint(c, "id"))
	response.Data(c, tenantMenuMap(m))
}

func MenuUpdateStatus(c *gin.Context) {
	tdb(c).Model(&model.TenantSystemMenu{}).Where("id = ?", httpx.Uint(c, "id")).Update("is_disable", httpx.Int(c, "is_disable"))
	response.Success(c, "修改成功", nil)
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
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort,
			"create_time": util.FormatDateTime(r.CreateTime), "menu_id": menuIDs,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func RoleAdd(c *gin.Context) {
	r := model.TenantSystemRole{Name: httpx.Str(c, "name"), Desc: httpx.Str(c, "desc"), Sort: httpx.Int(c, "sort"), TenantID: tenantDB(c), CreateTime: util.NowUnix()}
	tdb(c).Create(&r)
	for _, id := range httpx.Uints(c, "menu_id") {
		tdb(c).Create(&model.TenantSystemRoleMenu{RoleID: r.ID, MenuID: id})
	}
	response.Success(c, "添加成功", nil)
}

func RoleEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	tdb(c).Model(&model.TenantSystemRole{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "desc": httpx.Str(c, "desc"), "sort": httpx.Int(c, "sort"),
	})
	tdb(c).Where("role_id = ?", id).Delete(&model.TenantSystemRoleMenu{})
	for _, mid := range httpx.Uints(c, "menu_id") {
		tdb(c).Create(&model.TenantSystemRoleMenu{RoleID: id, MenuID: mid})
	}
	response.Success(c, "修改成功", nil)
}

func RoleDelete(c *gin.Context) {
	now := util.NowUnix()
	tdb(c).Model(&model.TenantSystemRole{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func RoleDetail(c *gin.Context) {
	var r model.TenantSystemRole
	tdb(c).First(&r, httpx.Uint(c, "id"))
	var menuIDs []uint
	tdb(c).Model(&model.TenantSystemRoleMenu{}).Where("role_id = ?", r.ID).Pluck("menu_id", &menuIDs)
	response.Data(c, gin.H{"id": r.ID, "name": r.Name, "desc": r.Desc, "sort": r.Sort, "menu_id": menuIDs})
}

func RoleAll(c *gin.Context) {
	var rows []model.TenantSystemRole
	db := tdb(c).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Find(&rows)
	response.Data(c, rows)
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
