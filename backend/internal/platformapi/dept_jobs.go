package platformapi

import (
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func DeptLists(c *gin.Context) {
	db := bootstrap.DB.Model(&model.Dept{}).Where("delete_time IS NULL")
	if name := httpx.QueryRaw(c, "name"); !util.PHPEmpty(name) {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if util.PHPIsset(httpx.Query(c), "status") && httpx.QueryRaw(c, "status") != "" {
		db = db.Where("status = ?", util.ParseInt(httpx.QueryRaw(c, "status")))
	}
	var rows []model.Dept
	db.Order("sort desc, id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	root := 0
	for i, d := range rows {
		maps = append(maps, deptMap(d))
		if i == 0 || int(d.Pid) < root {
			root = int(d.Pid)
		}
	}
	response.SuccessSilent(c, "", util.DeptTree(maps, root))
}

func DeptLeader(c *gin.Context) {
	var rows []model.Dept
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		out = append(out, map[string]any{"id": d.ID, "name": d.Name})
	}
	response.SuccessSilent(c, "", out)
}

func DeptAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	// PHP sceneAdd: pid require|integer|checkDept before name.
	if msg := util.DeptPidCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !deptExists(httpx.BodyUint(c, "pid")) {
		response.Fail(c, "部门不存在")
		return
	}
	if msg := util.DeptWriteCheckTaken(p, false, func(name string) bool {
		return deptNameTaken(0, name)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	d := model.Dept{
		Name: httpx.BodyRaw(c, "name"), Pid: httpx.BodyUint(c, "pid"), Sort: httpx.BodyInt(c, "sort"),
		Leader: httpx.BodyRaw(c, "leader"), Mobile: httpx.BodyRaw(c, "mobile"), Status: httpx.BodyInt(c, "status"),
		CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if err := bootstrap.DB.Create(&d).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "添加成功")
}

func DeptEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !util.PHPRequired(httpx.Body(c), "id") {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var cur model.Dept
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	if msg := util.DeptWriteCheckTaken(p, true, func(name string) bool {
		return deptNameTaken(id, name)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	pid := httpx.BodyUint(c, "pid")
	if cur.Pid == 0 {
		pid = 0
	} else {
		if id == pid {
			response.Fail(c, "上级部门不可是当前部门")
			return
		}
		if !deptExists(pid) {
			response.Fail(c, "部门不存在")
			return
		}
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Dept{}).Where("id = ? AND delete_time IS NULL", id).Updates(map[string]any{
		"name": httpx.BodyRaw(c, "name"), "pid": pid, "sort": httpx.BodyInt(c, "sort"),
		"leader": httpx.BodyRaw(c, "leader"), "mobile": httpx.BodyRaw(c, "mobile"), "status": httpx.BodyInt(c, "status"),
		"update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func DeptDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var cur model.Dept
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	var child int64
	bootstrap.DB.Model(&model.Dept{}).Where("pid = ? AND delete_time IS NULL", id).Count(&child)
	if child > 0 {
		response.Fail(c, "已关联下级部门,暂不可删除")
		return
	}
	var admins int64
	bootstrap.DB.Model(&model.AdminDept{}).Where("dept_id = ?", id).Count(&admins)
	if admins > 0 {
		response.Fail(c, "已关联管理员，暂不可删除")
		return
	}
	if cur.Pid == 0 {
		response.Fail(c, "顶级部门不可删除")
		return
	}
	bootstrap.DB.Model(&model.Dept{}).Where("id = ? AND delete_time IS NULL", id).Updates(util.SoftDeleteFields(util.NowUnix()))
	response.SuccessNotice(c, "删除成功")
}

func DeptDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	var d model.Dept
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")).First(&d).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	response.Data(c, deptRaw(d))
}

func DeptAll(c *gin.Context) {
	// PHP DeptLogic::getAllData uses request()->param('tenant_id').
	if tid, ok := httpx.ParamTenantID(c); ok {
		if tid == 0 {
			response.Data(c, []any{})
			return
		}
		db := tenantdb.ForTenant(tid)
		if db == nil {
			db = bootstrap.DB
		}
		var rows []model.TenantDept
		q := db.Where("delete_time IS NULL AND status = 1")
		q = q.Where("tenant_id = ?", tid)
		q.Order("sort desc, id desc").Find(&rows)
		if len(rows) == 0 {
			response.Data(c, []any{})
			return
		}
		maps := make([]map[string]any, 0, len(rows))
		root := int(rows[0].Pid)
		for _, d := range rows {
			maps = append(maps, tenantDeptRaw(d))
			if int(d.Pid) < root {
				root = int(d.Pid)
			}
		}
		response.Data(c, util.DeptTree(maps, root))
		return
	}
	var rows []model.Dept
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Order("sort desc, id desc").Find(&rows)
	if len(rows) == 0 {
		response.Data(c, []any{})
		return
	}
	maps := make([]map[string]any, 0, len(rows))
	root := int(rows[0].Pid)
	for _, d := range rows {
		maps = append(maps, deptRaw(d))
		if int(d.Pid) < root {
			root = int(d.Pid)
		}
	}
	response.Data(c, util.DeptTree(maps, root))
}

func JobsLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := bootstrap.DB.Model(&model.Jobs{}).Where("delete_time IS NULL")
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if code := lists.Param(q, "code"); code != "" {
		db = db.Where("code = ?", code)
	}
	if lists.Param(q, "status") != "" {
		db = db.Where("status = ?", lists.ParamInt(q, "status"))
	}
	var count int64
	db.Count(&count)
	var rows []model.Jobs
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, j := range rows {
		out = append(out, jobsMap(j))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func JobsAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.JobsWriteCheckTaken(p, false, func(name string) bool {
		return jobsNameTaken(0, name)
	}, func(code string) bool {
		return jobsCodeTaken(0, code)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	j := model.Jobs{
		Name: httpx.BodyRaw(c, "name"), Code: httpx.BodyRaw(c, "code"), Sort: httpx.BodyInt(c, "sort"),
		Status: httpx.BodyInt(c, "status"), Remark: httpx.BodyRaw(c, "remark"), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if err := bootstrap.DB.Create(&j).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "添加成功")
}

func JobsEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !util.PHPRequired(httpx.Body(c), "id") {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var exist model.Jobs
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&exist).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	if msg := util.JobsWriteCheckTaken(p, true, func(name string) bool {
		return jobsNameTaken(id, name)
	}, func(code string) bool {
		return jobsCodeTaken(id, code)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Jobs{}).Where("id = ? AND delete_time IS NULL", id).Updates(map[string]any{
		"name": httpx.BodyRaw(c, "name"), "code": httpx.BodyRaw(c, "code"), "sort": httpx.BodyInt(c, "sort"),
		"status": httpx.BodyInt(c, "status"), "remark": httpx.BodyRaw(c, "remark"), "update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func JobsDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var exist model.Jobs
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&exist).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	var used int64
	bootstrap.DB.Model(&model.AdminJobs{}).Where("jobs_id = ?", id).Count(&used)
	if used > 0 {
		response.Fail(c, "已关联管理员，暂不可删除")
		return
	}
	bootstrap.DB.Model(&model.Jobs{}).Where("id = ? AND delete_time IS NULL", id).Updates(util.SoftDeleteFields(util.NowUnix()))
	response.SuccessNotice(c, "删除成功")
}

func JobsDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	var j model.Jobs
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")).First(&j).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	response.Data(c, jobsRaw(j))
}

func JobsAll(c *gin.Context) {
	// PHP JobsLogic::getAllData uses request()->param('tenant_id').
	if tid, ok := httpx.ParamTenantID(c); ok {
		if tid == 0 {
			response.Data(c, []any{})
			return
		}
		db := tenantdb.ForTenant(tid)
		if db == nil {
			db = bootstrap.DB
		}
		var rows []model.TenantJobs
		q := db.Where("delete_time IS NULL AND status = 1").Where("tenant_id = ?", tid)
		q.Order("sort desc, id desc").Find(&rows)
		out := make([]map[string]any, 0, len(rows))
		for _, j := range rows {
			out = append(out, tenantJobsRaw(j))
		}
		response.Data(c, out)
		return
	}
	var rows []model.Jobs
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, j := range rows {
		out = append(out, jobsRaw(j))
	}
	response.Data(c, out)
}

func deptExists(id uint) bool {
	var n int64
	bootstrap.DB.Model(&model.Dept{}).Where("id = ? AND delete_time IS NULL", id).Count(&n)
	return n > 0
}

func deptNameTaken(id uint, name string) bool {
	var n int64
	q := bootstrap.DB.Model(&model.Dept{}).Where("name = ? AND delete_time IS NULL", name)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func jobsNameTaken(id uint, name string) bool {
	var n int64
	q := bootstrap.DB.Model(&model.Jobs{}).Where("name = ? AND delete_time IS NULL", name)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func jobsCodeTaken(id uint, code string) bool {
	var n int64
	q := bootstrap.DB.Model(&model.Jobs{}).Where("code = ? AND delete_time IS NULL", code)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func tenantDeptRaw(d model.TenantDept) map[string]any {
	return map[string]any{
		"id": d.ID, "name": d.Name, "pid": d.Pid, "sort": d.Sort, "leader": d.Leader,
		"mobile": d.Mobile, "status": d.Status, "tenant_id": d.TenantID,
		"create_time": util.FormatDateTime(d.CreateTime),
		"update_time": util.FormatDateTimeOrNil(d.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(d.DeleteTime),
	}
}

func deptRaw(d model.Dept) map[string]any {
	return map[string]any{
		"id": d.ID, "name": d.Name, "pid": d.Pid, "sort": d.Sort, "leader": d.Leader,
		"mobile": d.Mobile, "status": d.Status,
		"create_time": util.FormatDateTime(d.CreateTime),
		"update_time": util.FormatDateTimeOrNil(d.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(d.DeleteTime),
	}
}

func deptMap(d model.Dept) map[string]any {
	statusDesc := "停用"
	if d.Status == 1 {
		statusDesc = "正常"
	}
	out := deptRaw(d)
	out["status_desc"] = statusDesc
	return out
}

func tenantJobsRaw(j model.TenantJobs) map[string]any {
	return map[string]any{
		"id": j.ID, "name": j.Name, "code": j.Code, "sort": j.Sort, "status": j.Status,
		"remark": j.Remark, "tenant_id": j.TenantID, "create_time": util.FormatDateTime(j.CreateTime),
		"update_time": util.FormatDateTimeOrNil(j.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(j.DeleteTime),
	}
}

func jobsRaw(j model.Jobs) map[string]any {
	return map[string]any{
		"id": j.ID, "name": j.Name, "code": j.Code, "sort": j.Sort, "status": j.Status,
		"remark": j.Remark, "create_time": util.FormatDateTime(j.CreateTime),
		"update_time": util.FormatDateTimeOrNil(j.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(j.DeleteTime),
	}
}

func jobsMap(j model.Jobs) map[string]any {
	desc := "正常"
	if j.Status != 1 {
		desc = "停用"
	}
	out := jobsRaw(j)
	out["status_desc"] = desc
	return out
}
