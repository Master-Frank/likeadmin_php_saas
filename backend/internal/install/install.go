package install

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const installedMsg = "可能已经安装过本系统了，请删除配置目录下面的install.lock文件再尝试"

func Status(c *gin.Context) {
	lock := config.C.App.InstallLock
	installed := lock != ""
	if lock != "" {
		if _, err := os.Stat(lock); err != nil {
			installed = false
		}
	}
	response.Data(c, gin.H{"installed": installed, "lock": lock})
}

func Run(c *gin.Context) {
	lock := config.C.App.InstallLock
	if lock != "" {
		if _, err := os.Stat(lock); err == nil {
			response.Fail(c, installedMsg)
			return
		}
	}
	p := httpx.Params(c)
	if msg := CheckParams(p); msg != "" {
		response.Fail(c, msg)
		return
	}

	host := firstNonEmpty(p, "host", "hostname", "db_host")
	if host == "" {
		host = "127.0.0.1"
	}
	port := httpx.Int(c, "port")
	if port == 0 {
		port = httpx.Int(c, "hostport")
	}
	if port == 0 {
		port = httpx.Int(c, "db_port")
	}
	if port == 0 {
		port = 3306
	}
	dbName := firstNonEmpty(p, "name", "database")
	user := firstNonEmpty(p, "user", "username")
	pass := pick(p, "password")
	if dbName == "" || user == "" {
		response.Fail(c, "请填写数据库连接信息")
		return
	}
	if strings.Contains(dbName, ".") {
		response.Fail(c, "没有发现数据库信息")
		return
	}
	prefix := pick(p, "prefix")
	clearDB := isOn(p, "clear_db")
	importTest := isOn(p, "import_test_data")
	adminUser := pick(p, "admin_user")
	adminPass := pick(p, "admin_password")

	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=false&loc=Local",
		user, pass, host, port)
	db, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{})
	if err != nil {
		response.Fail(c, "安装错误，请检查连接信息:"+trimErr(err.Error()))
		return
	}

	var exists int
	_ = db.Raw("SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&exists)
	if exists == 0 {
		if err := db.Exec("CREATE DATABASE `" + dbName + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci").Error; err != nil {
			response.Fail(c, "创建数据库错误")
			return
		}
	} else if tableExists(db, dbName, prefix) && !clearDB {
		response.Fail(c, "数据表已存在，您之前可能已安装本系统，如需继续安装请选择新的数据库。")
		return
	} else if exists > 0 && clearDB {
		if err := db.Exec("DROP DATABASE `" + dbName + "`").Error; err != nil {
			response.Fail(c, "数据表已经存在，删除已存在库错误,请手动清除")
			return
		}
		if err := db.Exec("CREATE DATABASE `" + dbName + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci").Error; err != nil {
			response.Fail(c, "创建数据库错误!")
			return
		}
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=false&loc=Local&multiStatements=true",
		user, pass, host, port, dbName)
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		response.Fail(c, "安装错误，请检查连接信息:"+trimErr(err.Error()))
		return
	}

	imported := 0
	salt := ""
	if httpx.Int(c, "skip_sql") != 1 {
		sqlPath := FindLikeSQL(config.C.App.PublicDir)
		if sqlPath == "" {
			response.Fail(c, "创建表格失败")
			return
		}
		raw, err := os.ReadFile(sqlPath)
		if err != nil {
			response.Fail(c, "创建表格失败")
			return
		}
		imported, err = ImportSQL(db, string(raw), prefix)
		if err != nil {
			response.Fail(c, "创建表格失败")
			return
		}
		salt, err = insertAdmin(db, prefix, adminUser, adminPass, nowUnix())
		if err != nil {
			response.Fail(c, "创建表格失败")
			return
		}
	}
	if importTest {
		if err := importDemo(db, config.C.App.PublicDir, prefix, dbName); err != nil {
			response.Fail(c, err.Error())
			return
		}
	}
	if lock == "" {
		lock = filepath.Join(config.C.App.PublicDir, "../config/install.lock")
	}
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		response.Fail(c, err.Error())
		return
	}
	envPath := filepath.Join(filepath.Dir(lock), "..", ".env")
	if httpx.Str(c, "env_path") != "" {
		envPath = httpx.Str(c, "env_path")
	}
	_ = WriteEnv(envPath, host, dbName, user, pass, port, prefix, ctxutilHost(c), salt)
	content := fmt.Sprintf("installed_at=%s\nhost=%s\nport=%d\ndatabase=%s\n",
		time.Now().Format(time.RFC3339), host, port, dbName)
	if err := os.WriteFile(lock, []byte(content), 0o644); err != nil {
		response.Fail(c, "写入安装锁失败："+err.Error())
		return
	}
	restoreIndexLock()
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	response.Success(c, "安装成功", gin.H{"lock": lock, "imported": imported, "env": envPath})
}

func firstNonEmpty(p map[string]any, keys ...string) string {
	for _, k := range keys {
		if v := pick(p, k); v != "" {
			return v
		}
	}
	return ""
}

func trimErr(s string) string {
	runes := []rune(s)
	if len(runes) > 30 {
		return string(runes[:30]) + "..."
	}
	return s
}

func ctxutilHost(c *gin.Context) string {
	if h := c.Request.Host; h != "" {
		return h
	}
	return "127.0.0.1"
}

func CheckPort(host string, port int) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}
