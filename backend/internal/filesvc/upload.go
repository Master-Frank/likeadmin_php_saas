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

func FetchWechatAvatar(c *gin.Context, openid, headimg string) string {
	def := config.C.Project.DefaultImage["user_avatar"]
	if strings.TrimSpace(headimg) == "" {
		return def
	}
	name := util.MD5(openid+util.ToString(time.Now().Unix())) + ".jpeg"
	rel := filepath.ToSlash(filepath.Join("uploads/user/avatar", name))
	if _, err := storage.Fetch(c, headimg, rel); err != nil {
		return def
	}
	return rel
}

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

func CateChildIDs(db *gorm.DB, model any, parent uint, tenantID uint) []uint {
	var children []uint
	q := cateDB(db).Model(model).Where("pid = ? AND delete_time IS NULL", parent)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	q.Pluck("id", &children)
	out := append([]uint{}, children...)
	for _, id := range children {
		out = append(out, CateChildIDs(db, model, id, tenantID)...)
	}
	return out
}

func CateIDsInclusive(db *gorm.DB, model any, id uint, tenantID uint) []uint {
	return append(CateChildIDs(db, model, id, tenantID), id)
}

// FileIDsExist reports whether every id is a live (not soft-deleted) file row.
func FileIDsExist(db *gorm.DB, ids []uint) bool {
	if len(ids) == 0 {
		return false
	}
	seen := map[uint]struct{}{}
	uniq := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			return false
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniq = append(uniq, id)
	}
	var n int64
	db.Where("id IN ? AND delete_time IS NULL", uniq).Count(&n)
	return int(n) == len(uniq)
}

func ApplyFileCID(db *gorm.DB, cateModel any, params map[string]any, tenantID uint) *gorm.DB {
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
	ids := CateIDsInclusive(db, cateModel, cid, tenantID)
	return db.Where("cid IN ?", ids)
}

func DeleteStored(c *gin.Context, uris ...string) {
	for _, uri := range uris {
		_ = storage.Delete(c, uri)
	}
}
