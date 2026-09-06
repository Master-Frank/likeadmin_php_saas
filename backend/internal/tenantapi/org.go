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
	db := tdb(c).Model(&model.TenantDept{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if name := httpx.Str(c, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	var rows []model.TenantDept
	db.Order("sort desc, id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	root := 0
	for i, d := range rows {
		statusDesc := "停用"
		if d.Status == 1 {
			statusDesc = "正常"
		}
		maps = append(maps, map[string]any{
			"id": d.ID, "name": d.Name, "pid": d.Pid, "sort": d.Sort, "leader": d.Leader,
			"mobile": d.Mobile, "status": d.Status, "status_desc": statusDesc,
			"create_time": util.FormatDateTime(d.CreateTime),
			"update_time": util.FormatDateTimePtr(d.UpdateTime),
			"delete_time": util.FormatDateTimePtr(d.DeleteTime),
		})
		if i == 0 || int(d.Pid) < root {
			root = int(d.Pid)
		}
	}
	response.Success(c, "", util.DeptTree(maps, root))
}

func DeptLeader(c *gin.Context) {
	var rows []model.TenantDept
	db := tdb(c).Where("delete_time IS NULL AND status = 1")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc").Find(&rows)
	response.SuccessSilent(c, "", rows)
}

func DeptAdd(c *gin.Context) {
	tdb(c).Create(&model.TenantDept{
		Name: httpx.Str(c, "name"), Pid: httpx.Uint(c, "pid"), Sort: httpx.Int(c, "sort"),
		Leader: httpx.Str(c, "leader"), Mobile: httpx.Str(c, "mobile"), Status: httpx.Int(c, "status"),
		TenantID: tenantDB(c), CreateTime: util.NowUnix(),
	})
	response.Success(c, "添加成功", nil)
}

func DeptEdit(c *gin.Context) {
	tdb(c).Model(&model.TenantDept{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "pid": httpx.Uint(c, "pid"), "sort": httpx.Int(c, "sort"),
		"leader": httpx.Str(c, "leader"), "mobile": httpx.Str(c, "mobile"), "status": httpx.Int(c, "status"),
	})
	response.Success(c, "修改成功", nil)
}

func DeptDelete(c *gin.Context) {
	now := util.NowUnix()
	tdb(c).Model(&model.TenantDept{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func DeptDetail(c *gin.Context) {
	var d model.TenantDept
	tdb(c).First(&d, httpx.Uint(c, "id"))
	response.Data(c, d)
}

func DeptAll(c *gin.Context) {
	var rows []model.TenantDept
	db := tdb(c).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		maps = append(maps, map[string]any{"id": d.ID, "pid": d.Pid, "name": d.Name})
	}
	response.Data(c, util.LinearToTree(maps, "children", "id", "pid", 0))
}

func JobsLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.TenantJobs{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var count int64
	db.Count(&count)
	var rows []model.TenantJobs
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func JobsAdd(c *gin.Context) {
	tdb(c).Create(&model.TenantJobs{
		Name: httpx.Str(c, "name"), Code: httpx.Str(c, "code"), Sort: httpx.Int(c, "sort"),
		Status: httpx.Int(c, "status"), Remark: httpx.Str(c, "remark"), TenantID: tenantDB(c), CreateTime: util.NowUnix(),
	})
	response.Success(c, "添加成功", nil)
}

func JobsEdit(c *gin.Context) {
	tdb(c).Model(&model.TenantJobs{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "code": httpx.Str(c, "code"), "sort": httpx.Int(c, "sort"),
		"status": httpx.Int(c, "status"), "remark": httpx.Str(c, "remark"),
	})
	response.Success(c, "修改成功", nil)
}

func JobsDelete(c *gin.Context) {
	now := util.NowUnix()
	tdb(c).Model(&model.TenantJobs{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func JobsDetail(c *gin.Context) {
	var j model.TenantJobs
	tdb(c).First(&j, httpx.Uint(c, "id"))
	response.Data(c, j)
}

func JobsAll(c *gin.Context) {
	var rows []model.TenantJobs
	db := tdb(c).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Find(&rows)
	response.Data(c, rows)
}
