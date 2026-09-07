package filesvc

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/cfgsvc"
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

func FetchWechatAvatar(c *gin.Context, openid, headimg string) (string, error) {
	def := cfgsvc.GetString(c, "default_image", "user_avatar", config.C.Project.DefaultImage["user_avatar"])
	if strings.TrimSpace(headimg) == "" {
		return def, nil
	}
	name := util.MD5(openid+util.ToString(time.Now().Unix())) + ".jpeg"
	rel := filepath.ToSlash(filepath.Join("uploads/user/avatar", name))
	if _, err := storage.Fetch(c, headimg, rel); err != nil {
		engine := "local"
		if c != nil {
			engine = cfgsvc.GetString(c, "storage", "default", "local")
		}
		if engine != "" && engine != "local" {
			return "", fmt.Errorf("头像保存失败:%s", err.Error())
		}
		// PHP download_file returns '' when the local write is empty/failed.
		return "", nil
	}
	return rel, nil
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
	src, err := fh.Open()
	if err != nil {
		return "", "", err.Error()
	}
	defer src.Close()
	tmp, err := os.CreateTemp("", "likeadmin-upload-*")
	if err != nil {
		return "", "", err.Error()
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = io.Copy(tmp, src); err != nil {
		_ = tmp.Close()
		return "", "", err.Error()
	}
	if err = tmp.Close(); err != nil {
		return "", "", err.Error()
	}
	saved := buildSaveName(tmpPath, ext)
	rel = filepath.ToSlash(filepath.Join(dir, time.Now().Format("20060102"), saved))
	f, err := os.Open(tmpPath)
	if err != nil {
		return "", "", err.Error()
	}
	defer f.Close()
	if _, err = storage.Save(c, rel, f, fh.Size, fh.Header.Get("Content-Type")); err != nil {
		return "", "", err.Error()
	}
	return name, rel, ""
}

// buildSaveName mirrors PHP Server::buildSaveName:
// date('YmdHis') + substr(md5(realPath), 0, 5) + str_pad(rand(0, 9999), 4, '0') + .ext
func buildSaveName(realPath, ext string) string {
	return time.Now().Format("20060102150405") + util.MD5(realPath)[:5] + fmt.Sprintf("%04d", rand.Intn(10000)) + "." + ext
}

func UploadCID(c *gin.Context) uint {
	if v := c.PostForm("cid"); v != "" {
		return uint(util.ParseInt(v))
	}
	// PHP UploadController reads post('cid', 0) only — never the query string.
	return httpx.BodyUint(c, "cid")
}

// UploadCateOK rejects a non-zero cid that is missing or belongs to another tenant.
func UploadCateOK(db *gorm.DB, cateModel any, cid, tenantID uint) string {
	if cid == 0 {
		return ""
	}
	if db == nil {
		return "文件分类不存在"
	}
	q := cateDB(db).Model(cateModel).Where("id = ? AND delete_time IS NULL", cid)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	var n int64
	q.Count(&n)
	if n == 0 {
		return "文件分类不存在"
	}
	return ""
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
		rel := SetFileURL(c, uri)
		_ = storage.Delete(c, storage.ObjectKey(rel))
	}
}
