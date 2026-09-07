package platformapi

import (
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
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AdminLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := bootstrap.DB.Model(&model.Admin{}).Where("delete_time IS NULL")
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if account := lists.Param(q, "account"); account != "" {
		db = db.Where("account LIKE ?", "%"+account+"%")
	}
	if rid := lists.Param(q, "role_id"); rid != "" {
		var ids []uint
		bootstrap.DB.Model(&model.AdminRole{}).Where("role_id = ?", lists.ParamInt(q, "role_id")).Pluck("admin_id", &ids)
		if len(ids) > 0 {
			db = db.Where("id IN ?", ids)
		}
	}
	var count int64
	db.Count(&count)
	order := lists.OrderSQL(q, "id desc", map[string]bool{"create_time": true, "id": true})
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

func AdminAll(c *gin.Context) {
	var rows []model.Admin
	bootstrap.DB.Where("delete_time IS NULL").Order("id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "account": a.Account})
	}
	response.Data(c, out)
}

func AdminAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.AuthAdminAddCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	account := httpx.BodyStr(c, "account")
	name := httpx.BodyStr(c, "name")
	password := httpx.BodyStr(c, "password")
	var exist model.Admin
	if bootstrap.DB.Where("account = ? AND delete_time IS NULL", account).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	if bootstrap.DB.Where("name = ? AND delete_time IS NULL", name).First(&exist).Error == nil {
		response.Fail(c, "名称已存在")
		return
	}
	now := util.NowUnix()
	avatar := filesvc.SetFileURL(c, httpx.BodyStr(c, "avatar"))
	if avatar == "" {
		avatar = config.C.Project.DefaultImage["admin_avatar"]
	}
	admin := model.Admin{
		Name:            name,
		Account:         account,
		Avatar:          avatar,
		Password:        util.CreatePassword(password, config.C.Project.UniqueIdentification),
		CreateTime:      now,
		Disable:         adminAddDisable(p),
		MultipointLogin: httpx.BodyInt(c, "multipoint_login"),
	}
	roles, depts, jobs := httpx.BodyUints(c, "role_id"), httpx.BodyUints(c, "dept_id"), httpx.BodyUints(c, "jobs_id")
	if msg := platformAdminLinksCheck(roles, depts, jobs); msg != "" {
		response.Fail(c, msg)
		return
	}
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&admin).Error; err != nil {
			return err
		}
		return saveAdminLinks(tx, admin.ID, roles, depts, jobs)
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func AdminEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !authAdminIDPresent(p) {
		response.Fail(c, "管理员id不能为空")
		return
	}
	id := httpx.BodyUint(c, "id")
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	if msg := util.AuthAdminEditCheck(p, admin.Root == 1); msg != "" {
		response.Fail(c, msg)
		return
	}
	account := httpx.BodyStr(c, "account")
	name := httpx.BodyStr(c, "name")
	var exist model.Admin
	if bootstrap.DB.Where("account = ? AND delete_time IS NULL AND id <> ?", account, id).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	if bootstrap.DB.Where("name = ? AND delete_time IS NULL AND id <> ?", name, id).First(&exist).Error == nil {
		response.Fail(c, "名称已存在")
		return
	}
	now := util.NowUnix()
	avatar := ""
	if v := httpx.BodyStr(c, "avatar"); v != "" {
		avatar = filesvc.SetFileURL(c, v)
	}
	data := map[string]any{
		"name":             name,
		"account":          account,
		"disable":          httpx.BodyInt(c, "disable"),
		"multipoint_login": httpx.BodyInt(c, "multipoint_login"),
		"avatar":           avatar,
		"update_time":      now,
	}
	if pwd := httpx.BodyStr(c, "password"); pwd != "" {
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	var oldRoles []uint
	bootstrap.DB.Model(&model.AdminRole{}).Where("admin_id = ?", id).Pluck("role_id", &oldRoles)
	newRoles := httpx.BodyUints(c, "role_id")
	depts, jobs := httpx.BodyUints(c, "dept_id"), httpx.BodyUints(c, "jobs_id")
	if msg := platformAdminLinksCheck(newRoles, depts, jobs); msg != "" {
		response.Fail(c, msg)
		return
	}
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&admin).Updates(data).Error; err != nil {
			return err
		}
		tx.Where("admin_id = ?", id).Delete(&model.AdminRole{})
		tx.Where("admin_id = ?", id).Delete(&model.AdminDept{})
		tx.Where("admin_id = ?", id).Delete(&model.AdminJobs{})
		return saveAdminLinks(tx, id, newRoles, depts, jobs)
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	if httpx.BodyInt(c, "disable") == 1 || util.UintSlicesChanged(oldRoles, newRoles) {
		expireAdminTokens(id)
	}
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func AdminDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !authAdminIDPresent(httpx.Body(c)) {
		response.Fail(c, "管理员id不能为空")
		return
	}
	id := httpx.BodyUint(c, "id")
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
	cache.ClearAdminAuthCache(id)
	response.SuccessNotice(c, "操作成功")
}

func AdminDetail(c *gin.Context) {
	if !authAdminIDPresent(httpx.Query(c)) {
		response.Fail(c, "管理员id不能为空")
		return
	}
	id := httpx.QueryUint(c, "id")
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
	roleIDs, deptIDs, jobIDs := adminRelations(admin.ID)
	response.Data(c, gin.H{
		"user": gin.H{
			"id":               admin.ID,
			"account":          admin.Account,
			"name":             admin.Name,
			"avatar":           filesvc.GetFileURL(c, firstNonEmpty(admin.Avatar, config.C.Project.DefaultImage["admin_avatar"])),
			"disable":          admin.Disable,
			"root":             admin.Root,
			"multipoint_login": admin.MultipointLogin,
			"role_id":          roleIDs,
			"dept_id":          deptIDs,
			"jobs_id":          jobIDs,
		},
		"menu":        menu,
		"permissions": perms,
	})
}

func AdminEditSelf(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.AdminEditSelfCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	meta := ctxutil.Get(c)
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", meta.AdminID).First(&admin).Error != nil {
		response.Fail(c, "管理员不存在")
		return
	}
	data := map[string]any{
		"name":        httpx.BodyStr(c, "name"),
		"avatar":      filesvc.SetFileURL(c, httpx.BodyStr(c, "avatar")),
		"update_time": util.NowUnix(),
	}
	if pwd := httpx.BodyStr(c, "password"); pwd != "" {
		old := httpx.BodyStr(c, "password_old")
		if admin.Password != util.CreatePassword(old, config.C.Project.UniqueIdentification) {
			response.Fail(c, "当前密码错误")
			return
		}
		data["password"] = util.CreatePassword(pwd, config.C.Project.UniqueIdentification)
	}
	bootstrap.DB.Model(&admin).Updates(data)
	response.SuccessNotice(c, "操作成功")
}

func authAdminIDPresent(p map[string]any) bool {
	v, ok := p["id"]
	if !ok || v == nil {
		return false
	}
	return strings.TrimSpace(util.ToString(v)) != ""
}

func adminAddDisable(p map[string]any) int {
	if _, ok := p["disable"]; !ok {
		return 0
	}
	return util.ToInt(p["disable"])
}

func platformAdminLinksCheck(roles, depts, jobs []uint) string {
	if !platformIDsOwned(&model.SystemRole{}, roles, "delete_time IS NULL") {
		return "角色不存在"
	}
	if !platformIDsOwned(&model.Dept{}, depts, "delete_time IS NULL") {
		return "部门不存在"
	}
	if !platformIDsOwned(&model.Jobs{}, jobs, "delete_time IS NULL") {
		return "岗位不存在"
	}
	return ""
}

func platformIDsOwned(dest any, ids []uint, extra string) bool {
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
	q := bootstrap.DB.Model(dest).Where("id IN ?", uniq)
	if extra != "" {
		q = q.Where(extra)
	}
	var n int64
	q.Count(&n)
	return n == int64(len(uniq))
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
