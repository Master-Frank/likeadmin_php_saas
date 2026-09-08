package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// protectedDBs must never be dropped or overwritten by install.
var protectedDBs = map[string]bool{
	"localhost_likeadmin": true,
	"mysql":               true,
	"information_schema":  true,
	"performance_schema":  true,
	"sys":                 true,
}

// Options is the PHP install.php step-4 pipeline, without HTTP.
type Options struct {
	Host, User, Password, Name, Prefix string
	Port                               int
	ClearDB, ImportTest, SkipSQL       bool
	AdminUser, AdminPassword           string
	PublicDir, LockPath, EnvPath       string
	GoConfigPath, HTTPHost             string
	Now                                int64
}

// Result is what POST /install returns in data.
type Result struct {
	Lock     string
	Env      string
	Imported int
	Salt     string
}

// Apply creates the database, imports like.sql, seeds the root admin, and writes
// env/lock. It never touches protected pairing databases.
func Apply(opt Options) (*Result, error) {
	if msg := CheckParams(map[string]any{
		"prefix": opt.Prefix, "admin_user": opt.AdminUser,
		"admin_password": opt.AdminPassword, "admin_confirm_password": opt.AdminPassword,
	}); msg != "" {
		return nil, fmt.Errorf("%s", msg)
	}
	if opt.Host == "" {
		opt.Host = "127.0.0.1"
	}
	if opt.Port == 0 {
		opt.Port = 3306
	}
	if opt.Prefix == "" {
		opt.Prefix = "la_"
	}
	if opt.Name == "" || opt.User == "" {
		return nil, fmt.Errorf("请填写数据库连接信息")
	}
	if strings.Contains(opt.Name, ".") {
		return nil, fmt.Errorf("没有发现数据库信息")
	}
	if !identOK(opt.Name) || !identOK(opt.Prefix) {
		return nil, fmt.Errorf("没有发现数据库信息")
	}
	if protectedDBs[strings.ToLower(opt.Name)] {
		return nil, fmt.Errorf("拒绝操作受保护数据库 %s", opt.Name)
	}
	if opt.Now == 0 {
		opt.Now = nowUnix()
	}
	if err := CheckPort(opt.Host, opt.Port); err != nil {
		return nil, fmt.Errorf("安装错误，请检查连接信息:%s", trimErr(err.Error()))
	}

	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=false&loc=Local",
		opt.User, opt.Password, opt.Host, opt.Port)
	db, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("安装错误，请检查连接信息:%s", trimErr(err.Error()))
	}
	setPHPSQLMode(db)
	if err := ensureDatabase(db, opt.Name, opt.Prefix, opt.ClearDB); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=false&loc=Local&multiStatements=true",
		opt.User, opt.Password, opt.Host, opt.Port, opt.Name)
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("安装错误，请检查连接信息:%s", trimErr(err.Error()))
	}
	setPHPSQLMode(db)

	imported := 0
	salt := ""
	if !opt.SkipSQL {
		raw, err := ReadLikeSQL(opt.PublicDir)
		if err != nil || len(raw) == 0 {
			return nil, fmt.Errorf("创建表格失败")
		}
		imported, err = ImportSQL(db, string(raw), opt.Prefix, opt.Name)
		if err != nil {
			return nil, fmt.Errorf("创建表格失败")
		}
		salt, err = insertAdmin(db, opt.Prefix, opt.AdminUser, opt.AdminPassword, opt.Now)
		if err != nil {
			return nil, fmt.Errorf("创建表格失败")
		}
	}
	if opt.ImportTest {
		if err := importDemo(db, opt.PublicDir, opt.Prefix, opt.Name); err != nil {
			return nil, err
		}
	}

	lock := opt.LockPath
	if lock == "" && opt.PublicDir != "" {
		lock = filepath.Join(opt.PublicDir, "../config/install.lock")
	}
	envPath := opt.EnvPath
	if envPath == "" && lock != "" {
		envPath = filepath.Join(filepath.Dir(lock), "..", ".env")
	}
	if envPath != "" {
		if err := WriteEnv(envPath, opt.Host, opt.Name, opt.User, opt.Password, opt.Port, opt.Prefix, opt.HTTPHost, salt); err != nil {
			return nil, fmt.Errorf("写入环境配置失败：%w", err)
		}
	}
	if opt.GoConfigPath != "" {
		if err := WriteGoConfig(opt.GoConfigPath, opt.Host, opt.Name, opt.User, opt.Password, opt.Port, opt.Prefix, opt.HTTPHost, salt); err != nil {
			return nil, err
		}
	}
	if lock != "" {
		if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(lock, []byte{}, 0o644); err != nil {
			return nil, fmt.Errorf("写入安装锁失败：%w", err)
		}
	}
	if opt.PublicDir != "" {
		restoreIndexLockDir(opt.PublicDir)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	return &Result{Lock: lock, Env: envPath, Imported: imported, Salt: salt}, nil
}

func ensureDatabase(db *gorm.DB, dbName, prefix string, clearDB bool) error {
	var exists int
	_ = db.Raw("SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&exists)
	if exists == 0 {
		if err := db.Exec("CREATE DATABASE `" + dbName + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci").Error; err != nil {
			return fmt.Errorf("创建数据库错误")
		}
		return nil
	}
	if tableExists(db, dbName, prefix) && !clearDB {
		return fmt.Errorf("数据表已存在，您之前可能已安装本系统，如需继续安装请选择新的数据库。")
	}
	if exists > 0 && clearDB {
		if err := db.Exec("DROP DATABASE `" + dbName + "`").Error; err != nil {
			return fmt.Errorf("数据表已经存在，删除已存在库错误,请手动清除")
		}
		if err := db.Exec("CREATE DATABASE `" + dbName + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci").Error; err != nil {
			return fmt.Errorf("创建数据库错误!")
		}
	}
	return nil
}

func restoreIndexLockDir(pub string) {
	for _, dir := range []string{"admin", "mobile"} {
		restoreIndexFile(filepath.Join(pub, dir))
	}
}
