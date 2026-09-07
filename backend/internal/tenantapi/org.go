package tenantapi

import (
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func DeptLists(c *gin.Context) {
	tid, ok := requireTenant(c)
	if !ok {
		response.SuccessSilent(c, "", []any{})
		return
	}
	db := tdb(c).Model(&model.TenantDept{}).Where("delete_time IS NULL AND tenant_id = ?", tid)
	if name := httpx.QueryStr(c, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if status := httpx.QueryStr(c, "status"); status != "" {
		db = db.Where("status = ?", util.ParseInt(status))
	}
	var rows []model.TenantDept
	db.Order("sort desc, id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	root := 0
	for i, d := range rows {
		maps = append(maps, tenantDeptMap(d))
		if i == 0 || int(d.Pid) < root {
			root = int(d.Pid)
		}
	}
	response.SuccessSilent(c, "", util.DeptTree(maps, root))
}

func DeptLeader(c *gin.Context) {
	tid, ok := requireTenant(c)
	if !ok {
		response.SuccessSilent(c, "", []any{})
		return
	}
	var rows []model.TenantDept
	db := tdb(c).Where("delete_time IS NULL AND status = 1 AND tenant_id = ?", tid)
	db.Order("sort desc, id desc").Find(&rows)
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
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.DeptPidCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !tenantDeptExists(c, httpx.BodyUint(c, "pid")) {
		response.Fail(c, "部门不存在")
		return
	}
	if msg := util.DeptWriteCheckTaken(p, false, func(name string) bool {
		return tenantDeptNameTaken(c, 0, name)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Create(&model.TenantDept{
		Name: httpx.BodyStr(c, "name"), Pid: httpx.BodyUint(c, "pid"), Sort: httpx.BodyInt(c, "sort"),
		Leader: httpx.BodyStr(c, "leader"), Mobile: httpx.BodyStr(c, "mobile"), Status: httpx.BodyInt(c, "status"),
		TenantID: tenantDB(c), CreateTime: util.NowUnix(),
	})
	response.SuccessNotice(c, "添加成功")
}

func DeptEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !httpx.BodyHas(c, "id") || httpx.BodyStr(c, "id") == "" {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var cur model.TenantDept
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&cur).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	if msg := util.DeptWriteCheckTaken(p, true, func(name string) bool {
		return tenantDeptNameTaken(c, id, name)
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
		if !tenantDeptExists(c, pid) {
			response.Fail(c, "部门不存在")
			return
		}
	}
	scopeTID(tdb(c).Model(&model.TenantDept{}).Where("id = ?", id), c).Updates(map[string]any{
		"name": httpx.BodyStr(c, "name"), "pid": pid, "sort": httpx.BodyInt(c, "sort"),
		"leader": httpx.BodyStr(c, "leader"), "mobile": httpx.BodyStr(c, "mobile"), "status": httpx.BodyInt(c, "status"),
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
	var cur model.TenantDept
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&cur).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	var child int64
	scopeTID(tdb(c).Model(&model.TenantDept{}).Where("pid = ? AND delete_time IS NULL", id), c).Count(&child)
	if child > 0 {
		response.Fail(c, "已关联下级部门,暂不可删除")
		return
	}
	var admins int64
	tdb(c).Model(&model.TenantAdminDept{}).Where("dept_id = ?", id).Count(&admins)
	if admins > 0 {
		response.Fail(c, "已关联管理员，暂不可删除")
		return
	}
	if cur.Pid == 0 {
		response.Fail(c, "顶级部门不可删除")
		return
	}
	scopeTID(tdb(c).Model(&model.TenantDept{}).Where("id = ?", id), c).Update("delete_time", util.NowUnix())
	response.SuccessNotice(c, "删除成功")
}

func DeptDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	var d model.TenantDept
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")), c).First(&d).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	response.Data(c, tenantDeptRaw(d))
}

func DeptAll(c *gin.Context) {
	tid, ok := requireTenant(c)
	if !ok {
		response.Data(c, []any{})
		return
	}
	var rows []model.TenantDept
	db := tdb(c).Where("delete_time IS NULL AND status = 1 AND tenant_id = ?", tid)
	db.Order("sort desc, id desc").Find(&rows)
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
}

func JobsLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	if listsNeedTenant(c, q) {
		return
	}
	db := tdb(c).Model(&model.TenantJobs{}).Where("delete_time IS NULL AND tenant_id = ?", tenantDB(c))
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
	var rows []model.TenantJobs
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, j := range rows {
		out = append(out, tenantJobsMap(j))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func JobsAdd(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.JobsWriteCheckTaken(p, false, func(name string) bool {
		return tenantJobsNameTaken(c, 0, name)
	}, func(code string) bool {
		return tenantJobsCodeTaken(c, 0, code)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Create(&model.TenantJobs{
		Name: httpx.BodyStr(c, "name"), Code: httpx.BodyStr(c, "code"), Sort: httpx.BodyInt(c, "sort"),
		Status: httpx.BodyInt(c, "status"), Remark: httpx.BodyStr(c, "remark"), TenantID: tenantDB(c), CreateTime: util.NowUnix(),
	})
	response.SuccessNotice(c, "添加成功")
}

func JobsEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if !httpx.BodyHas(c, "id") || httpx.BodyStr(c, "id") == "" {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "id")
	var exist model.TenantJobs
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&exist).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	if msg := util.JobsWriteCheckTaken(p, true, func(name string) bool {
		return tenantJobsNameTaken(c, id, name)
	}, func(code string) bool {
		return tenantJobsCodeTaken(c, id, code)
	}); msg != "" {
		response.Fail(c, msg)
		return
	}
	scopeTID(tdb(c).Model(&model.TenantJobs{}).Where("id = ?", id), c).Updates(map[string]any{
		"name": httpx.BodyStr(c, "name"), "code": httpx.BodyStr(c, "code"), "sort": httpx.BodyInt(c, "sort"),
		"status": httpx.BodyInt(c, "status"), "remark": httpx.BodyStr(c, "remark"),
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
	var exist model.TenantJobs
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c).First(&exist).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	var used int64
	tdb(c).Model(&model.TenantAdminJobs{}).Where("jobs_id = ?", id).Count(&used)
	if used > 0 {
		response.Fail(c, "已关联管理员，暂不可删除")
		return
	}
	scopeTID(tdb(c).Model(&model.TenantJobs{}).Where("id = ?", id), c).Update("delete_time", util.NowUnix())
	response.SuccessNotice(c, "删除成功")
}

func JobsDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	var j model.TenantJobs
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")), c).First(&j).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	response.Data(c, tenantJobsRaw(j))
}

func tenantDeptExists(c *gin.Context, id uint) bool {
	var n int64
	scopeTID(tdb(c).Model(&model.TenantDept{}).Where("id = ? AND delete_time IS NULL", id), c).Count(&n)
	return n > 0
}

func tenantDeptNameTaken(c *gin.Context, id uint, name string) bool {
	var n int64
	q := scopeTID(tdb(c).Model(&model.TenantDept{}).Where("name = ? AND delete_time IS NULL", name), c)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func tenantJobsNameTaken(c *gin.Context, id uint, name string) bool {
	var n int64
	q := scopeTID(tdb(c).Model(&model.TenantJobs{}).Where("name = ? AND delete_time IS NULL", name), c)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func tenantJobsCodeTaken(c *gin.Context, id uint, code string) bool {
	var n int64
	q := scopeTID(tdb(c).Model(&model.TenantJobs{}).Where("code = ? AND delete_time IS NULL", code), c)
	if id > 0 {
		q = q.Where("id <> ?", id)
	}
	q.Count(&n)
	return n > 0
}

func JobsAll(c *gin.Context) {
	tid, ok := requireTenant(c)
	if !ok {
		response.Data(c, []any{})
		return
	}
	var rows []model.TenantJobs
	db := tdb(c).Where("delete_time IS NULL AND status = 1 AND tenant_id = ?", tid)
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, j := range rows {
		out = append(out, tenantJobsRaw(j))
	}
	response.Data(c, out)
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

func tenantDeptMap(d model.TenantDept) map[string]any {
	statusDesc := "停用"
	if d.Status == 1 {
		statusDesc = "正常"
	}
	out := tenantDeptRaw(d)
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

func tenantJobsMap(j model.TenantJobs) map[string]any {
	desc := "停用"
	if j.Status == 1 {
		desc = "正常"
	}
	out := tenantJobsRaw(j)
	out["status_desc"] = desc
	return out
}
