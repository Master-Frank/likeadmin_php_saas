package filesvc

import (
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/storage"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	SourceAdmin = 0
	SourceUser  = 1
)

func ReceiveUpload(c *gin.Context, scene, dir string) (name, rel, errMsg string) {
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		return "", "", "未找到上传文件的信息"
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fh.Filename), "."))
	if msg := util.UploadExtCheck(scene, ext, config.C.Project.FileImage, config.C.Project.FileVideo, config.C.Project.FileFile); msg != "" {
		return "", "", msg
	}
	name = truncateUploadName(fh.Filename)
	saved := time.Now().Format("20060102150405") + util.MD5(fh.Filename)[:8] + "." + ext
	rel = filepath.ToSlash(filepath.Join(dir, time.Now().Format("20060102"), saved))
	src, err := fh.Open()
	if err != nil {
		return "", "", err.Error()
	}
	defer src.Close()
	if _, err = storage.Save(c, rel, src, fh.Size, fh.Header.Get("Content-Type")); err != nil {
		return "", "", err.Error()
	}
	return name, rel, ""
}

func UploadCID(c *gin.Context) uint {
	if v := c.PostForm("cid"); v != "" {
		return uint(util.ParseInt(v))
	}
	return httpx.Uint(c, "cid")
}

func truncateUploadName(name string) string {
	if len(name) <= 128 {
		return name
	}
	if len(name) < 5 {
		return name[:128]
	}
	return name[:123] + name[len(name)-5:]
}

func cateDB(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{NewDB: true})
}

func CateChildIDs(db *gorm.DB, model any, parent uint) []uint {
	var children []uint
	cateDB(db).Model(model).Where("pid = ? AND delete_time IS NULL", parent).Pluck("id", &children)
	out := append([]uint{}, children...)
	for _, id := range children {
		out = append(out, CateChildIDs(db, model, id)...)
	}
	return out
}

func CateIDsInclusive(db *gorm.DB, model any, id uint) []uint {
	return append(CateChildIDs(db, model, id), id)
}

func ApplyFileCID(db *gorm.DB, cateModel any, params map[string]any) *gorm.DB {
	v, ok := params["cid"]
	if !ok {
		return db
	}
	if util.ToString(v) == "0" {
		return db.Where("cid = ?", 0)
	}
	if strings.TrimSpace(util.ToString(v)) == "" {
		return db
	}
	cid := uint(util.ToInt(v))
	if cid == 0 {
		return db
	}
	ids := CateIDsInclusive(db, cateModel, cid)
	return db.Where("cid IN ?", ids)
}

func DeleteStored(c *gin.Context, uris ...string) {
	for _, uri := range uris {
		_ = storage.Delete(c, uri)
	}
}
