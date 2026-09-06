package tenantapi

import (
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func FileLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.TenantFile{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if t := lists.ParamInt(q, "type"); t > 0 {
		db = db.Where("type = ?", t)
	}
	if lists.Param(q, "source") != "" {
		db = db.Where("source = ?", lists.ParamInt(q, "source"))
	}
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	db = filesvc.ApplyFileCID(db, &model.TenantFileCate{}, q.Params)
	var count int64
	db.Count(&count)
	var rows []model.TenantFile
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, f := range rows {
		out = append(out, map[string]any{
			"id": f.ID, "cid": f.Cid, "type": f.Type, "name": f.Name, "uri": f.URI,
			"create_time": util.FormatDateTime(f.CreateTime),
			"url":         filesvc.GetFileURL(c, f.URI),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func FileMove(c *gin.Context) {
	p := httpx.Params(c)
	ids := httpx.Uints(c, "ids")
	if msg := util.FileMoveCheck(p, ids); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&model.TenantFile{}).Where("id IN ?", ids).Updates(map[string]any{
		"cid": httpx.Uint(c, "cid"), "update_time": now,
	})
	response.SuccessNotice(c, "移动成功")
}

func FileRename(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.FileRenameCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&model.TenantFile{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "update_time": now,
	})
	response.SuccessNotice(c, "重命名成功")
}

func FileDelete(c *gin.Context) {
	p := httpx.Params(c)
	ids := httpx.Uints(c, "ids")
	if msg := util.FileDeleteCheck(p, ids); msg != "" {
		response.Fail(c, msg)
		return
	}
	var rows []model.TenantFile
	tdb(c).Where("id IN ? AND delete_time IS NULL", ids).Find(&rows)
	uris := make([]string, 0, len(rows))
	for _, row := range rows {
		uris = append(uris, row.URI)
	}
	filesvc.DeleteStored(c, uris...)
	tdb(c).Unscoped().Where("id IN ?", ids).Delete(&model.TenantFile{})
	response.SuccessNotice(c, "删除成功")
}

func FileListCate(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.TenantFileCate{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if t := lists.ParamInt(q, "type"); t > 0 {
		db = db.Where("type = ?", t)
	}
	var rows []model.TenantFileCate
	db.Order("id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		maps = append(maps, map[string]any{"id": r.ID, "pid": r.Pid, "type": r.Type, "name": r.Name})
	}
	tree := util.LinearToTree(maps, "children", "id", "pid", 0)
	response.Lists(c, tree, int64(len(rows)), q.PageNo, q.PageSize, nil)
}

func FileAddCate(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.FileAddCateCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	row := model.TenantFileCate{
		Type: httpx.Int(c, "type"), Pid: httpx.Uint(c, "pid"), Name: httpx.Str(c, "name"),
		TenantID: tenantDB(c), CreateTime: util.NowUnix(),
	}
	tdb(c).Create(&row)
	response.SuccessNotice(c, "添加成功")
}

func FileEditCate(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.FileEditCateCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Model(&model.TenantFileCate{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "编辑成功")
}

func FileDelCate(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.FileIDCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	ids := filesvc.CateIDsInclusive(tdb(c), &model.TenantFileCate{}, id)
	var files []model.TenantFile
	tdb(c).Where("cid IN ? AND delete_time IS NULL", ids).Find(&files)
	fileIDs := make([]uint, 0, len(files))
	uris := make([]string, 0, len(files))
	for _, f := range files {
		fileIDs = append(fileIDs, f.ID)
		uris = append(uris, f.URI)
	}
	if len(fileIDs) > 0 {
		filesvc.DeleteStored(c, uris...)
		tdb(c).Unscoped().Where("id IN ?", fileIDs).Delete(&model.TenantFile{})
	}
	tdb(c).Unscoped().Where("id IN ?", ids).Delete(&model.TenantFileCate{})
	response.SuccessNotice(c, "删除成功")
}

func UploadImage(c *gin.Context) { tenantUpload(c, 10, "uploads/images", "image") }
func UploadVideo(c *gin.Context) { tenantUpload(c, 20, "uploads/video", "video") }
func UploadFile(c *gin.Context)  { tenantUpload(c, 30, "uploads/file", "file") }

func tenantUpload(c *gin.Context, typ int, dir, scene string) {
	name, rel, errMsg := filesvc.ReceiveUpload(c, scene, dir)
	if errMsg != "" {
		response.Fail(c, errMsg)
		return
	}
	row := model.TenantFile{
		Cid: filesvc.UploadCID(c), Type: typ, Name: name, URI: rel, Source: filesvc.SourceAdmin,
		TenantID: ctxutil.Get(c).TenantID, CreateTime: util.NowUnix(),
	}
	tdb(c).Create(&row)
	response.Success(c, "上传成功", gin.H{
		"id": row.ID, "cid": row.Cid, "type": row.Type, "name": row.Name,
		"uri": filesvc.GetFileURL(c, rel), "url": rel,
	})
}
