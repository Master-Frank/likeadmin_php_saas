package tenantapi

import (
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func FileLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	if listsNeedTenant(c, q) {
		return
	}
	db := tdb(c).Model(&model.TenantFile{}).Where("delete_time IS NULL AND tenant_id = ?", tenantDB(c))
	if lists.HasParam(q, "type") {
		db = db.Where("type = ?", lists.ParamInt(q, "type"))
	}
	if lists.Param(q, "source") != "" {
		db = db.Where("source = ?", lists.ParamInt(q, "source"))
	}
	if name := lists.Param(q, "name"); name != "" {
		db = db.Where("name LIKE ?", "%"+name+"%")
	}
	db = filesvc.ApplyFileCID(db, &model.TenantFileCate{}, q.Params, tenantDB(c))
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
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	ids := httpx.BodyUints(c, "ids")
	if msg := util.FileMoveCheck(p, ids); msg != "" {
		response.Fail(c, msg)
		return
	}
	// PHP FileLogic::move uses SoftDelete File, so deleted rows stay put.
	now := util.NowUnix()
	scopeTID(tdb(c).Model(&model.TenantFile{}).Where("id IN ? AND delete_time IS NULL", ids), c).Updates(map[string]any{
		"cid": httpx.BodyUint(c, "cid"), "update_time": now,
	})
	response.SuccessNotice(c, "移动成功")
}

func FileRename(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.FileRenameCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	scopeTID(tdb(c).Model(&model.TenantFile{}).Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "id")), c).Updates(map[string]any{
		"name": httpx.BodyStr(c, "name"), "update_time": now,
	})
	response.SuccessNotice(c, "重命名成功")
}

func FileDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	ids := httpx.BodyUints(c, "ids")
	if msg := util.FileDeleteCheck(p, ids); msg != "" {
		response.Fail(c, msg)
		return
	}
	var rows []model.TenantFile
	scopeTID(tdb(c).Where("id IN ? AND delete_time IS NULL", ids), c).Find(&rows)
	uris := make([]string, 0, len(rows))
	for _, row := range rows {
		uris = append(uris, row.URI)
	}
	filesvc.DeleteStored(c, uris...)
	scopeTID(tdb(c).Model(&model.TenantFile{}).Where("id IN ? AND delete_time IS NULL", ids), c).Updates(util.SoftDeleteFields(util.NowUnix()))
	response.SuccessNotice(c, "删除成功")
}

func FileListCate(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	if listsNeedTenant(c, q) {
		return
	}
	db := tdb(c).Model(&model.TenantFileCate{}).Where("delete_time IS NULL AND tenant_id = ?", tenantDB(c))
	if lists.HasParam(q, "type") {
		db = db.Where("type = ?", lists.ParamInt(q, "type"))
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
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.FileAddCateCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	now := util.NowUnix()
	row := model.TenantFileCate{
		Type: httpx.BodyInt(c, "type"), Pid: httpx.BodyUint(c, "pid"), Name: httpx.BodyStr(c, "name"),
		TenantID: tenantDB(c), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	tdb(c).Create(&row)
	response.SuccessNotice(c, "添加成功")
}

func FileEditCate(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.FileEditCateCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	scopeTID(tdb(c).Model(&model.TenantFileCate{}).Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "id")), c).Updates(map[string]any{
		"name": httpx.BodyStr(c, "name"), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "编辑成功")
}

func FileDelCate(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.FileIDCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.BodyUint(c, "id")
	// PHP FileLogic::delCate updates by id with no existence check.
	ids := filesvc.CateIDsInclusive(tdb(c), &model.TenantFileCate{}, id, tenantDB(c))
	var files []model.TenantFile
	scopeTID(tdb(c).Where("cid IN ? AND delete_time IS NULL", ids), c).Find(&files)
	fileIDs := make([]uint, 0, len(files))
	uris := make([]string, 0, len(files))
	for _, f := range files {
		fileIDs = append(fileIDs, f.ID)
		uris = append(uris, f.URI)
	}
	now := util.NowUnix()
	if len(fileIDs) > 0 {
		filesvc.DeleteStored(c, uris...)
		scopeTID(tdb(c).Model(&model.TenantFile{}).Where("id IN ? AND delete_time IS NULL", fileIDs), c).Updates(util.SoftDeleteFields(now))
	}
	scopeTID(tdb(c).Model(&model.TenantFileCate{}).Where("id IN ? AND delete_time IS NULL", ids), c).Updates(util.SoftDeleteFields(now))
	response.SuccessNotice(c, "删除成功")
}

func UploadImage(c *gin.Context) { tenantUpload(c, 10, "uploads/images", "image") }
func UploadVideo(c *gin.Context) { tenantUpload(c, 20, "uploads/video", "video") }
func UploadFile(c *gin.Context)  { tenantUpload(c, 30, "uploads/file", "file") }

func tenantUpload(c *gin.Context, typ int, dir, scene string) {
	if !guardTenantWrite(c) {
		return
	}
	cid := filesvc.UploadCID(c)
	if msg := filesvc.UploadCateOK(tdb(c), &model.TenantFileCate{}, cid, tenantDB(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	name, rel, errMsg := filesvc.ReceiveUpload(c, scene, dir)
	if errMsg != "" {
		response.Fail(c, errMsg)
		return
	}
	now := util.NowUnix()
	row := model.TenantFile{
		Cid: cid, Type: typ, Name: name, URI: rel, Source: filesvc.SourceAdmin,
		TenantID: tenantDB(c), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	tdb(c).Create(&row)
	response.Success(c, "上传成功", gin.H{
		"id": row.ID, "cid": row.Cid, "type": row.Type, "name": row.Name,
		"uri": filesvc.GetFileURL(c, rel), "url": rel,
	})
}
