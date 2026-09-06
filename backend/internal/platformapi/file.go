package platformapi

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
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
	db := bootstrap.DB.Model(&model.File{}).Where("delete_time IS NULL")
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
		cid := lists.ParamInt(q, "cid")
		db = db.Where("cid = ?", cid)
	}
	var count int64
	db.Count(&count)
	var rows []model.File
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
	ids := httpx.Uints(c, "ids")
	cid := httpx.Uint(c, "cid")
	if len(ids) > 0 {
		bootstrap.DB.Model(&model.File{}).Where("id IN ?", ids).Update("cid", cid)
	}
	response.Success(c, "移动成功", nil)
}

func FileRename(c *gin.Context) {
	bootstrap.DB.Model(&model.File{}).Where("id = ?", httpx.Uint(c, "id")).Update("name", httpx.Str(c, "name"))
	response.Success(c, "修改成功", nil)
}

func FileDelete(c *gin.Context) {
	ids := httpx.Uints(c, "ids")
	now := util.NowUnix()
	if len(ids) > 0 {
		bootstrap.DB.Model(&model.File{}).Where("id IN ?", ids).Update("delete_time", now)
	}
	response.Success(c, "删除成功", nil)
}

func FileListCate(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.FileCate{}).Where("delete_time IS NULL")
	if t := lists.ParamInt(q, "type"); t > 0 {
		db = db.Where("type = ?", t)
	}
	var rows []model.FileCate
	db.Order("id desc").Find(&rows)
	maps := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		maps = append(maps, map[string]any{"id": r.ID, "pid": r.Pid, "type": r.Type, "name": r.Name})
	}
	tree := util.LinearToTree(maps, "children", "id", "pid", 0)
	response.Lists(c, tree, int64(len(rows)), q.PageNo, q.PageSize, nil)
}

func FileAddCate(c *gin.Context) {
	row := model.FileCate{Type: httpx.Int(c, "type"), Pid: httpx.Uint(c, "pid"), Name: httpx.Str(c, "name"), CreateTime: util.NowUnix()}
	bootstrap.DB.Create(&row)
	response.Success(c, "添加成功", nil)
}

func FileEditCate(c *gin.Context) {
	bootstrap.DB.Model(&model.FileCate{}).Where("id = ?", httpx.Uint(c, "id")).Update("name", httpx.Str(c, "name"))
	response.Success(c, "修改成功", nil)
}

func FileDelCate(c *gin.Context) {
	id := httpx.Uint(c, "id")
	now := util.NowUnix()
	bootstrap.DB.Model(&model.FileCate{}).Where("id = ?", id).Update("delete_time", now)
	bootstrap.DB.Model(&model.File{}).Where("cid = ?", id).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func UploadImage(c *gin.Context) { uploadSave(c, 10, "uploads/images", config.C.Project.FileImage) }
func UploadVideo(c *gin.Context) { uploadSave(c, 20, "uploads/video", config.C.Project.FileVideo) }
func UploadFile(c *gin.Context)  { uploadSave(c, 30, "uploads/file", config.C.Project.FileFile) }

func uploadSave(c *gin.Context, typ int, dir string, allow []string) {
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
	abs := filepath.Join(config.C.App.PublicDir, rel)
	_ = os.MkdirAll(filepath.Dir(abs), 0755)
	src, err := fh.Open()
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	defer src.Close()
	dst, err := os.Create(abs)
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	defer dst.Close()
	if _, err = io.Copy(dst, src); err != nil {
		response.Fail(c, err.Error())
		return
	}
	cid := uint(0)
	if v := c.PostForm("cid"); v != "" {
		cid = uint(util.ParseInt(v))
	} else {
		cid = httpx.Uint(c, "cid")
	}
	row := model.File{Cid: cid, Type: typ, Name: fh.Filename, URI: rel, CreateTime: util.NowUnix()}
	bootstrap.DB.Create(&row)
	response.Success(c, "上传成功", gin.H{
		"id": row.ID, "cid": row.Cid, "type": row.Type, "name": row.Name,
		"uri": filesvc.GetFileURL(c, rel), "url": rel,
	})
}
