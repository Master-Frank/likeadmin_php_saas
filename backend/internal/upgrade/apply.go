package upgrade

import (
	"archive/zip"
	"crypto/md5"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantmenu"
	"likeadmin/backend/internal/tenantdb"

	"gorm.io/gorm"
)

const openBasedirMsg = "请临时关闭服务器本站点的跨域攻击设置，并重启 nginx、PHP，具体参考相关升级文档"

// CheckOpenBasedir mirrors PHP UpgradeLogic::upgrade open_basedir precheck.
// Go-only deploys have no php.ini, so this reads LIKEADMIN_OPEN_BASEDIR /
// PHP_OPEN_BASEDIR only — it does not exec `php -r ini_get(...)`.
func CheckOpenBasedir() error {
	basedir := os.Getenv("LIKEADMIN_OPEN_BASEDIR")
	if basedir == "" {
		basedir = os.Getenv("PHP_OPEN_BASEDIR")
	}
	if strings.Contains(basedir, "server") {
		return errStatus(openBasedirMsg)
	}
	return nil
}

// ApplyPackage downloads, extracts, and applies a likeadmin upgrade zip (SQL / menu / files).
func ApplyPackage(link, zipName string) error {
	if err := CheckOpenBasedir(); err != nil {
		return err
	}
	root := serverRoot()
	localDir := filepath.Join(root, "upgrade")
	tempDir := filepath.Join(localDir, "temp")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return err
	}
	savePath, err := resolvePackage(link, localDir)
	if err != nil {
		return err
	}
	_ = os.RemoveAll(tempDir)
	if err := unzip(savePath, tempDir); err != nil {
		return err
	}
	if err := applyExtracted(tempDir, filepath.Dir(root)+string(os.PathSeparator), backendRoot(), bootstrap.DB); err != nil {
		return err
	}
	_ = os.RemoveAll(tempDir)
	return nil
}

// ApplyLocal applies an already-downloaded upgrade zip (offline / fixture).
// versionNo, when set, is written to upgrade/version.json after a successful apply
// (same as the online UpgradeDo path). If omitted, a version is taken from the zip name.
func ApplyLocal(zipPath string, versionNo ...string) error {
	if err := ApplyPackage(zipPath, ""); err != nil {
		return err
	}
	ver := ""
	if len(versionNo) > 0 {
		ver = strings.TrimSpace(versionNo[0])
	}
	if ver == "" {
		ver = versionFromFilename(zipPath)
	}
	return WriteLocalVersion(ver)
}

var versionInName = regexp.MustCompile(`\d+\.\d+(?:\.\d+)*`)

func versionFromFilename(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return versionInName.FindString(base)
}

func resolvePackage(link, saveDir string) (string, error) {
	link = strings.TrimSpace(link)
	if link == "" {
		return "", errStatus("获取文件错误")
	}
	if strings.HasPrefix(link, "file://") {
		p := strings.TrimPrefix(link, "file://")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
		return "", errStatus("获取文件错误")
	}
	if !strings.Contains(link, "://") {
		if st, err := os.Stat(link); err == nil && !st.IsDir() {
			return link, nil
		}
	}
	return downFile(link, saveDir)
}

func applyExtracted(tempDir, projectDest, backendDest string, db *gorm.DB) error {
	applyFiles := func() error {
		if err := upgradePgSQL(filepath.Join(tempDir, "project", "pg")); err != nil {
			return err
		}
		if err := upgradeFile(filepath.Join(tempDir, "project", "server"), projectDest); err != nil {
			return err
		}
		return upgradeFile(filepath.Join(tempDir, "project", "backend"), backendDest)
	}
	if db == nil {
		return applyFiles()
	}
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := upgradeSQL(tx, filepath.Join(tempDir, "project", "sql", "data")); err != nil {
			return err
		}
		if err := upgradeMenu(tx, filepath.Join(tempDir, "project", "menu")); err != nil {
			return err
		}
		return applyFiles()
	}); err != nil {
		return err
	}
	return upgradeSQL(db, filepath.Join(tempDir, "project", "sql", "structure"))
}

func downFile(remote, saveDir string) (string, error) {
	resp, err := downloadClient.Get(remote)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", errStatus("获取文件错误")
	}
	name := filepath.Base(strings.Split(remote, "?")[0])
	if name == "" || name == "/" || name == "." {
		name = "package.zip"
	}
	path := filepath.Join(saveDir, name)
	base := filepath.Dir(path)
	if err := os.MkdirAll(base, 0777); err != nil {
		return "", err
	}
	_ = os.Chmod(base, 0777)
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	n, err := io.Copy(f, resp.Body)
	_ = f.Close()
	if err != nil {
		return "", err
	}
	// PHP file_put_contents on an empty 200 body returns 0 → false → 获取文件错误.
	if n == 0 {
		_ = os.Remove(path)
		return "", errStatus("获取文件错误")
	}
	// A 200 HTML/JSON error page must fail here (获取文件错误), not later at unzip.
	head := make([]byte, 4)
	hf, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return "", errStatus("获取文件错误")
	}
	_, _ = io.ReadFull(hf, head)
	_ = hf.Close()
	if !isZipMagic(head) {
		_ = os.Remove(path)
		return "", errStatus("获取文件错误")
	}
	return path, nil
}

func isZipMagic(b []byte) bool {
	if len(b) < 4 || b[0] != 'P' || b[1] != 'K' {
		return false
	}
	switch {
	case b[2] == 3 && b[3] == 4: // local file header
		return true
	case b[2] == 5 && b[3] == 6: // empty archive
		return true
	case b[2] == 7 && b[3] == 8: // spanned
		return true
	default:
		return false
	}
}

type applyError string

func (e applyError) Error() string { return string(e) }

func errStatus(msg string) error { return applyError(msg) }

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return applyError("解压文件错误")
	}
	defer r.Close()
	if err := os.MkdirAll(dest, 0755); err != nil {
		return applyError("解压文件错误")
	}
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if strings.HasPrefix(name, "..") {
			continue
		}
		target := filepath.Join(dest, name)
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return applyError("解压文件错误")
		}
		rc, err := f.Open()
		if err != nil {
			return applyError("解压文件错误")
		}
		out, err := os.Create(target)
		if err != nil {
			_ = rc.Close()
			return applyError("解压文件错误")
		}
		_, err = io.Copy(out, rc)
		_ = out.Close()
		_ = rc.Close()
		if err != nil {
			return applyError("解压文件错误")
		}
	}
	return nil
}

func upgradeSQL(db *gorm.DB, dir string) error {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return nil
	}
	prefix := config.Prefix()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return applyError("更新数据库数据失败")
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".sql") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil || len(raw) == 0 {
			continue
		}
		sql := strings.ReplaceAll(string(raw), "`la_", "`"+prefix)
		for _, stmt := range strings.Split(sql, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if err := db.Exec(stmt).Error; err != nil {
				if strings.Contains(dir, "structure") {
					return applyError("更新数据库结构失败")
				}
				return applyError("更新数据库数据失败")
			}
		}
	}
	return nil
}

// listUpgradeTenants mirrors PHP Tenant::query() SoftDelete: skip deleted rows.
func listUpgradeTenants(db *gorm.DB) []model.Tenant {
	var tenants []model.Tenant
	if db == nil {
		return tenants
	}
	db.Where("delete_time IS NULL").Find(&tenants)
	return tenants
}

func upgradeMenu(db *gorm.DB, dir string) error {
	if _, err := os.Stat(dir); err != nil {
		return nil
	}
	return upgradeMenusFor(db, listUpgradeTenants(db))
}

// upgradeMenusFor is PHP UpgradeLogic::upgradeMenu's per-tenant loop.
// Tests pass a disposable tenant so pairing tenants are not rewritten.
func upgradeMenusFor(db *gorm.DB, tenants []model.Tenant) error {
	for _, t := range tenants {
		tdb := tenantdb.ForTenantOn(db, t.ID)
		if err := tdb.Where("tenant_id = ?", t.ID).Delete(&model.TenantSystemMenu{}).Error; err != nil {
			return applyError("更新菜单信息失败")
		}
		if err := tenantmenu.Reinit(db, tdb, t.ID); err != nil {
			return applyError("更新菜单信息失败")
		}
	}
	cache.ClearAdminAuthCache(0)
	return nil
}

func upgradeFile(tempFile, oldFile string) error {
	tempFile = strings.TrimSpace(tempFile)
	oldFile = strings.TrimSpace(oldFile)
	if tempFile == "" || oldFile == "" {
		return applyError("更新文件失败")
	}
	if _, err := os.Stat(tempFile); err != nil {
		return nil
	}
	if err := os.MkdirAll(oldFile, 0777); err != nil {
		return applyError("更新文件失败")
	}
	return filepath.WalkDir(tempFile, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return applyError("更新文件失败")
		}
		rel, err := filepath.Rel(tempFile, path)
		if err != nil {
			return applyError("更新文件失败")
		}
		dest := filepath.Join(oldFile, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0777)
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return applyError("更新文件失败")
		}
		if cur, err := os.ReadFile(dest); err == nil {
			// PHP UpgradeLogic::upgradeFile skips when md5(src)==md5(dest).
			if md5.Sum(src) == md5.Sum(cur) {
				return nil
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0777); err != nil {
			return applyError("更新文件失败")
		}
		if err := os.WriteFile(dest, src, 0644); err != nil {
			return applyError("更新文件失败")
		}
		return nil
	})
}

func upgradePgSQL(dir string) error {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return applyError("更新PG数据库数据失败")
	}
	var files []string
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".sql") {
			continue
		}
		files = append(files, filepath.Join(dir, ent.Name()))
	}
	if len(files) == 0 {
		return nil
	}
	db, err := openPgsqlDB()
	if err != nil || db == nil {
		return applyError("更新PG数据库数据失败")
	}
	defer db.Close()
	prefix := config.C.Pgsql.Prefix
	if prefix == "" {
		prefix = "la_"
	}
	for _, path := range files {
		raw, err := os.ReadFile(path)
		if err != nil || len(raw) == 0 {
			continue
		}
		sqlText := strings.ReplaceAll(string(raw), "la_", prefix)
		for _, stmt := range strings.Split(sqlText, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return applyError("更新PG数据库数据失败")
			}
		}
	}
	return nil
}

func openPgsqlDB() (*sql.DB, error) {
	cfg := config.C.Pgsql
	if cfg.Hostname == "" || cfg.Database == "" {
		return nil, fmt.Errorf("pgsql not configured")
	}
	port := cfg.Hostport
	if port == 0 {
		port = 5432
	}
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Hostname, port, cfg.Username, cfg.Password, cfg.Database)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
