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
	if name := httpx.QueryStr(c, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if status := httpx.QueryStr(c, "status"); status != "" {
		db = db.Where("status = ?", util.ParseInt(status))
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
	p := httpx.Params(c)
	if msg := util.DeptWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !deptExists(httpx.Uint(c, "pid")) {
		response.Fail(c, "部门不存在")
		return
	}
	if deptNameTaken(0, httpx.Str(c, "name")) {
		response.Fail(c, "部门名称已存在")
		return
	}
	d := model.Dept{
		Name: httpx.Str(c, "name"), Pid: httpx.Uint(c, "pid"), Sort: httpx.Int(c, "sort"),
		Leader: httpx.Str(c, "leader"), Mobile: httpx.Str(c, "mobile"), Status: httpx.Int(c, "status"),
		CreateTime: util.NowUnix(),
	}
	if err := bootstrap.DB.Create(&d).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "添加成功")
}

func DeptEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.DeptWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	var cur model.Dept
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&cur).Error != nil {
		response.Fail(c, "当前部门信息缺失")
		return
	}
	pid := httpx.Uint(c, "pid")
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
	if deptNameTaken(id, httpx.Str(c, "name")) {
		response.Fail(c, "部门名称已存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Dept{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "pid": pid, "sort": httpx.Int(c, "sort"),
		"leader": httpx.Str(c, "leader"), "mobile": httpx.Str(c, "mobile"), "status": httpx.Int(c, "status"),
		"update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func DeptDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
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
	bootstrap.DB.Model(&model.Dept{}).Where("id = ?", id).Update("delete_time", util.NowUnix())
	response.SuccessNotice(c, "删除成功")
}

func DeptDetail(c *gin.Context) {
	if httpx.QueryUint(c, "id") == 0 {
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
	if _, ok := httpx.Params(c)["tenant_id"]; ok {
		tid := httpx.Uint(c, "tenant_id")
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
	p := httpx.Params(c)
	if msg := util.JobsWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if jobsNameTaken(0, httpx.Str(c, "name")) {
		response.Fail(c, "岗位名称已存在")
		return
	}
	if jobsCodeTaken(0, httpx.Str(c, "code")) {
		response.Fail(c, "岗位编码已存在")
		return
	}
	j := model.Jobs{
		Name: httpx.Str(c, "name"), Code: httpx.Str(c, "code"), Sort: httpx.Int(c, "sort"),
		Status: httpx.Int(c, "status"), Remark: httpx.Str(c, "remark"), CreateTime: util.NowUnix(),
	}
	if err := bootstrap.DB.Create(&j).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "添加成功")
}

func JobsEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.JobsWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	var exist model.Jobs
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", id).First(&exist).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	if jobsNameTaken(id, httpx.Str(c, "name")) {
		response.Fail(c, "岗位名称已存在")
		return
	}
	if jobsCodeTaken(id, httpx.Str(c, "code")) {
		response.Fail(c, "岗位编码已存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Jobs{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "code": httpx.Str(c, "code"), "sort": httpx.Int(c, "sort"),
		"status": httpx.Int(c, "status"), "remark": httpx.Str(c, "remark"), "update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func JobsDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
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
	bootstrap.DB.Model(&model.Jobs{}).Where("id = ?", id).Update("delete_time", util.NowUnix())
	response.SuccessNotice(c, "删除成功")
}

func JobsDetail(c *gin.Context) {
	if httpx.QueryUint(c, "id") == 0 {
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
	if _, ok := httpx.Params(c)["tenant_id"]; ok {
		tid := httpx.Uint(c, "tenant_id")
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
