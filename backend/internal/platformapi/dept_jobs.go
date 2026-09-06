package platformapi

import (
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func DeptLists(c *gin.Context) {
	db := bootstrap.DB.Model(&model.Dept{}).Where("delete_time IS NULL")
	if name := httpx.Str(c, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	if status := httpx.Str(c, "status"); status != "" {
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
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Order("sort desc").Find(&rows)
	response.SuccessSilent(c, "", rows)
}

func DeptAdd(c *gin.Context) {
	d := model.Dept{
		Name: httpx.Str(c, "name"), Pid: httpx.Uint(c, "pid"), Sort: httpx.Int(c, "sort"),
		Leader: httpx.Str(c, "leader"), Mobile: httpx.Str(c, "mobile"), Status: httpx.Int(c, "status"),
		CreateTime: util.NowUnix(),
	}
	if err := bootstrap.DB.Create(&d).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "添加成功", nil)
}

func DeptEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == httpx.Uint(c, "pid") {
		response.Fail(c, "上级部门不能是自己")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Dept{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "pid": httpx.Uint(c, "pid"), "sort": httpx.Int(c, "sort"),
		"leader": httpx.Str(c, "leader"), "mobile": httpx.Str(c, "mobile"), "status": httpx.Int(c, "status"),
		"update_time": now,
	})
	response.Success(c, "修改成功", nil)
}

func DeptDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var child int64
	bootstrap.DB.Model(&model.Dept{}).Where("pid = ? AND delete_time IS NULL", id).Count(&child)
	if child > 0 {
		response.Fail(c, "请先删除子部门")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Dept{}).Where("id = ?", id).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func DeptDetail(c *gin.Context) {
	var d model.Dept
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&d).Error != nil {
		response.Fail(c, "部门不存在")
		return
	}
	response.Data(c, deptMap(d))
}

func DeptAll(c *gin.Context) {
	var rows []model.Dept
	bootstrap.DB.Where("delete_time IS NULL").Order("sort desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, d := range rows {
		maps = append(maps, map[string]any{"id": d.ID, "pid": d.Pid, "name": d.Name})
	}
	response.Data(c, util.LinearToTree(maps, "children", "id", "pid", 0))
}

func JobsLists(c *gin.Context) {
	q := lists.Parse(c)
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
		desc := "正常"
		if j.Status != 1 {
			desc = "停用"
		}
		out = append(out, map[string]any{
			"id": j.ID, "name": j.Name, "code": j.Code, "sort": j.Sort, "status": j.Status,
			"remark": j.Remark, "create_time": util.FormatDateTime(j.CreateTime), "status_desc": desc,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func JobsAdd(c *gin.Context) {
	j := model.Jobs{
		Name: httpx.Str(c, "name"), Code: httpx.Str(c, "code"), Sort: httpx.Int(c, "sort"),
		Status: httpx.Int(c, "status"), Remark: httpx.Str(c, "remark"), CreateTime: util.NowUnix(),
	}
	if err := bootstrap.DB.Create(&j).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "添加成功", nil)
}

func JobsEdit(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Jobs{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "code": httpx.Str(c, "code"), "sort": httpx.Int(c, "sort"),
		"status": httpx.Int(c, "status"), "remark": httpx.Str(c, "remark"), "update_time": now,
	})
	response.Success(c, "修改成功", nil)
}

func JobsDelete(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Jobs{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func JobsDetail(c *gin.Context) {
	var j model.Jobs
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")).First(&j).Error != nil {
		response.Fail(c, "岗位不存在")
		return
	}
	response.Data(c, j)
}

func JobsAll(c *gin.Context) {
	var rows []model.Jobs
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Order("sort desc").Find(&rows)
	response.Data(c, rows)
}

func deptMap(d model.Dept) map[string]any {
	statusDesc := "停用"
	if d.Status == 1 {
		statusDesc = "正常"
	}
	return map[string]any{
		"id": d.ID, "name": d.Name, "pid": d.Pid, "sort": d.Sort, "leader": d.Leader,
		"mobile": d.Mobile, "status": d.Status, "status_desc": statusDesc,
		"create_time": util.FormatDateTime(d.CreateTime),
		"update_time": util.FormatDateTimePtr(d.UpdateTime),
		"delete_time": util.FormatDateTimePtr(d.DeleteTime),
	}
}
