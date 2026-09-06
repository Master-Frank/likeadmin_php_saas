package tenantapi

import (
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/storage"
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
	if _, ok := q.Params["cid"]; ok {
		db = db.Where("cid = ?", lists.ParamInt(q, "cid"))
	}
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
	tdb(c).Model(&model.TenantFile{}).Where("id IN ?", ids).Update("cid", httpx.Uint(c, "cid"))
	response.SuccessNotice(c, "移动成功")
}

func FileRename(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.FileRenameCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Model(&model.TenantFile{}).Where("id = ?", httpx.Uint(c, "id")).Update("name", httpx.Str(c, "name"))
	response.SuccessNotice(c, "重命名成功")
}

func FileDelete(c *gin.Context) {
	p := httpx.Params(c)
	ids := httpx.Uints(c, "ids")
	if msg := util.FileDeleteCheck(p, ids); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	tdb(c).Model(&model.TenantFile{}).Where("id IN ?", ids).Update("delete_time", now)
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
	tdb(c).Model(&model.TenantFileCate{}).Where("id = ?", httpx.Uint(c, "id")).Update("name", httpx.Str(c, "name"))
	response.SuccessNotice(c, "编辑成功")
}

func FileDelCate(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.FileIDCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	tdb(c).Model(&model.TenantFileCate{}).Where("id = ?", id).Update("delete_time", now)
	tdb(c).Model(&model.TenantFile{}).Where("cid = ?", id).Update("delete_time", now)
	response.SuccessNotice(c, "删除成功")
}

func UploadImage(c *gin.Context) { tenantUpload(c, 10, "uploads/images", config.C.Project.FileImage) }
func UploadVideo(c *gin.Context) { tenantUpload(c, 20, "uploads/video", config.C.Project.FileVideo) }
func UploadFile(c *gin.Context)  { tenantUpload(c, 30, "uploads/file", config.C.Project.FileFile) }

func tenantUpload(c *gin.Context, typ int, dir string, allow []string) {
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, "请选择文件")
		return
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fh.Filename), "."))
	ok := false
	for _, a := range allow {
		if a == ext {
			ok = true
			break
		}
	}
	if !ok {
		response.Fail(c, "不支持的文件类型")
		return
	}
	name := time.Now().Format("20060102150405") + util.MD5(fh.Filename)[:8] + "." + ext
	rel := filepath.ToSlash(filepath.Join(dir, time.Now().Format("20060102"), name))
	src, err := fh.Open()
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	defer src.Close()
	if _, err = storage.Save(c, rel, src, fh.Size, fh.Header.Get("Content-Type")); err != nil {
		response.Fail(c, err.Error())
		return
	}
	cid := uint(0)
	if v := c.PostForm("cid"); v != "" {
		cid = uint(util.ParseInt(v))
	} else {
		cid = httpx.Uint(c, "cid")
	}
	row := model.TenantFile{
		Cid: cid, Type: typ, Name: fh.Filename, URI: rel, Source: 2,
		TenantID: ctxutil.Get(c).TenantID, CreateTime: util.NowUnix(),
	}
	tdb(c).Create(&row)
	response.Success(c, "上传成功", gin.H{
		"id": row.ID, "cid": row.Cid, "type": row.Type, "name": row.Name,
		"uri": filesvc.GetFileURL(c, rel), "url": rel,
	})
}
