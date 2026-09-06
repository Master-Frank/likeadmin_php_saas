package platformapi

import (
	"likeadmin/backend/internal/bootstrap"
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
	db := bootstrap.DB.Model(&model.Admin{}).Where("delete_time IS NULL")
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if account := lists.Param(q, "account"); account != "" {
		db = db.Where("account LIKE ?", "%"+account+"%")
	}
	if rid := lists.ParamInt(q, "role_id"); rid > 0 {
		var ids []uint
		bootstrap.DB.Model(&model.AdminRole{}).Where("role_id = ?", rid).Pluck("admin_id", &ids)
		if len(ids) == 0 {
			response.Lists(c, []any{}, 0, q.PageNo, q.PageSize, nil)
			return
		}
		db = db.Where("id IN ?", ids)
	}
	var count int64
	db.Count(&count)
	order := "id desc"
	if q.Field != "" && (q.OrderBy == "asc" || q.OrderBy == "desc") {
		order = q.Field + " " + q.OrderBy
	}
	var rows []model.Admin
	db.Order(order).Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, adminListItem(c, a))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func adminListItem(c *gin.Context, a model.Admin) map[string]any {
	roleIDs, deptIDs, jobIDs := adminRelations(a.ID)
	roleName := joinNames(roleNames(roleIDs))
	if a.Root == 1 {
		roleName = "系统管理员"
	}
	disableDesc := "正常"
	if a.Disable == 1 {
		disableDesc = "停用"
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
		"avatar":           filesvc.GetFileURL(c, firstNonEmpty(a.Avatar, config.C.Project.DefaultImage["admin_avatar"])),
		"role_id":          roleIDs,
		"dept_id":          deptIDs,
		"jobs_id":          jobIDs,
		"disable_desc":     disableDesc,
		"role_name":        roleName,
		"dept_name":        joinNames(deptNames(deptIDs)),
		"jobs_name":        joinNames(jobNames(jobIDs)),
	}
}

func AdminAdd(c *gin.Context) {
	account := httpx.Str(c, "account")
	name := httpx.Str(c, "name")
	password := httpx.Str(c, "password")
	if account == "" || name == "" || password == "" {
		response.Fail(c, "参数缺失")
		return
	}
	var exist model.Admin
	if bootstrap.DB.Where("account = ? AND delete_time IS NULL", account).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	now := util.NowUnix()
	avatar := filesvc.SetFileURL(c, httpx.Str(c, "avatar"))
	if avatar == "" {
		avatar = config.C.Project.DefaultImage["admin_avatar"]
	}
	admin := model.Admin{
		Name:            name,
		Account:         account,
		Avatar:          avatar,
		Password:        util.CreatePassword(password, config.C.Project.UniqueIdentification),
		CreateTime:      now,
		Disable:         httpx.Int(c, "disable"),
		MultipointLogin: httpx.Int(c, "multipoint_login"),
	}
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		return saveAdminLinks(tx, admin.ID, httpx.Uints(c, "role_id"), httpx.Uints(c, "dept_id"), httpx.Uints(c, "jobs_id"))
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "添加成功", nil)
}

func AdminEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	now := util.NowUnix()
	data := map[string]any{
		"name":             httpx.Str(c, "name"),
		"account":          httpx.Str(c, "account"),
		"disable":          httpx.Int(c, "disable"),
		"multipoint_login": httpx.Int(c, "multipoint_login"),
		"avatar":           filesvc.SetFileURL(c, httpx.Str(c, "avatar")),
		"update_time":      now,
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&admin).Updates(data).Error; err != nil {
			return err
		}
		tx.Where("admin_id = ?", id).Delete(&model.AdminRole{})
		tx.Where("admin_id = ?", id).Delete(&model.AdminDept{})
		tx.Where("admin_id = ?", id).Delete(&model.AdminJobs{})
		return saveAdminLinks(tx, id, httpx.Uints(c, "role_id"), httpx.Uints(c, "dept_id"), httpx.Uints(c, "jobs_id"))
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	if httpx.Int(c, "disable") == 1 {
		expireAdminTokens(id)
	}
	response.Success(c, "修改成功", nil)
}

func AdminDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	if admin.Root == 1 {
		response.Fail(c, "超级管理员不允许被删除")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&admin).Update("delete_time", now)
	expireAdminTokens(id)
	bootstrap.DB.Where("admin_id = ?", id).Delete(&model.AdminRole{})
	bootstrap.DB.Where("admin_id = ?", id).Delete(&model.AdminDept{})
	bootstrap.DB.Where("admin_id = ?", id).Delete(&model.AdminJobs{})
	response.Success(c, "删除成功", nil)
}

func AdminDetail(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	roleIDs, deptIDs, jobIDs := adminRelations(id)
	response.Data(c, gin.H{
		"id":               admin.ID,
		"account":          admin.Account,
		"name":             admin.Name,
		"disable":          admin.Disable,
		"root":             admin.Root,
		"multipoint_login": admin.MultipointLogin,
		"avatar":           filesvc.GetFileURL(c, firstNonEmpty(admin.Avatar, config.C.Project.DefaultImage["admin_avatar"])),
		"role_id":          roleIDs,
		"dept_id":          deptIDs,
		"jobs_id":          jobIDs,
	})
}

func AdminMySelf(c *gin.Context) {
	meta := ctxutil.Get(c)
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", meta.AdminID).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	menu := menuTreeByAdmin(c, admin)
	perms := buttonPerms(admin)
	response.Data(c, gin.H{
		"user": gin.H{
			"id":       admin.ID,
			"account":  admin.Account,
			"name":     admin.Name,
			"avatar":   filesvc.GetFileURL(c, firstNonEmpty(admin.Avatar, config.C.Project.DefaultImage["admin_avatar"])),
			"disable":  admin.Disable,
			"root":     admin.Root,
		},
		"menu":        menu,
		"permissions": perms,
	})
}

func AdminEditSelf(c *gin.Context) {
	meta := ctxutil.Get(c)
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", meta.AdminID).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	data := map[string]any{
		"name":        httpx.Str(c, "name"),
		"avatar":      filesvc.SetFileURL(c, httpx.Str(c, "avatar")),
		"update_time": util.NowUnix(),
	}
	if pwd := httpx.Str(c, "password"); pwd != "" {
		old := httpx.Str(c, "password_old")
		if old != "" && admin.Password != util.CreatePassword(old, config.C.Project.UniqueIdentification) {
			response.Fail(c, "原密码错误")
			return
		}
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	bootstrap.DB.Model(&admin).Updates(data)
	response.Success(c, "修改成功", nil)
}

func saveAdminLinks(tx *gorm.DB, adminID uint, roles, depts, jobs []uint) error {
	for _, id := range roles {
		if err := tx.Create(&model.AdminRole{AdminID: adminID, RoleID: id}).Error; err != nil {
			return err
		}
	}
	for _, id := range depts {
		if err := tx.Create(&model.AdminDept{AdminID: adminID, DeptID: id}).Error; err != nil {
			return err
		}
	}
	for _, id := range jobs {
		if err := tx.Create(&model.AdminJobs{AdminID: adminID, JobsID: id}).Error; err != nil {
			return err
		}
	}
	return nil
}

func expireAdminTokens(adminID uint) {
	var sess []model.AdminSession
	bootstrap.DB.Where("admin_id = ?", adminID).Find(&sess)
	now := util.NowUnix()
	for _, s := range sess {
		bootstrap.DB.Model(&s).Updates(map[string]any{"expire_time": now, "update_time": now})
		cache.DeleteAdminInfo(s.Token)
	}
}

func adminRelations(id uint) (roles, depts, jobs []uint) {
	bootstrap.DB.Model(&model.AdminRole{}).Where("admin_id = ?", id).Pluck("role_id", &roles)
	bootstrap.DB.Model(&model.AdminDept{}).Where("admin_id = ?", id).Pluck("dept_id", &depts)
	bootstrap.DB.Model(&model.AdminJobs{}).Where("admin_id = ?", id).Pluck("jobs_id", &jobs)
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

func roleNames(ids []uint) []string {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.SystemRole
	bootstrap.DB.Where("id IN ?", ids).Find(&rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

func deptNames(ids []uint) []string {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.Dept
	bootstrap.DB.Where("id IN ?", ids).Find(&rows)
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

func jobNames(ids []uint) []string {
	if len(ids) == 0 {
		return nil
	}
	var rows []model.Jobs
	bootstrap.DB.Where("id IN ?", ids).Find(&rows)
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

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
