package install

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

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
			response.Fail(c, "程序已安装")
			return
		}
	}
	host := httpx.Str(c, "hostname")
	if host == "" {
		host = httpx.Str(c, "db_host")
	}
	port := httpx.Int(c, "hostport")
	if port == 0 {
		port = httpx.Int(c, "db_port")
	}
	if port == 0 {
		port = 3306
	}
	dbName := httpx.Str(c, "database")
	user := httpx.Str(c, "username")
	pass := httpx.Str(c, "password")
	if host == "" || dbName == "" || user == "" {
		response.Fail(c, "请填写数据库连接信息")
		return
	}
	prefix := httpx.Str(c, "prefix")
	if prefix == "" {
		prefix = config.Prefix()
	}
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s:%d)/?charset=utf8mb4&parseTime=false&loc=Local",
		user, pass, host, port)
	db, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{})
	if err != nil {
		response.Fail(c, "数据库连接失败："+err.Error())
		return
	}
	if err := db.Exec("CREATE DATABASE IF NOT EXISTS `" + dbName + "` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci").Error; err != nil {
		response.Fail(c, "创建数据库失败："+err.Error())
		return
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=false&loc=Local&multiStatements=true",
		user, pass, host, port, dbName)
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		response.Fail(c, "数据库连接失败："+err.Error())
		return
	}
	imported := 0
	if httpx.Int(c, "skip_sql") != 1 {
		sqlPath := FindLikeSQL(config.C.App.PublicDir)
		if sqlPath == "" {
			response.Fail(c, "未找到 like.sql")
			return
		}
		raw, err := os.ReadFile(sqlPath)
		if err != nil {
			response.Fail(c, "读取 like.sql 失败："+err.Error())
			return
		}
		imported, err = ImportSQL(db, string(raw), prefix)
		if err != nil {
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
	_ = WriteEnv(envPath, host, dbName, user, pass, port, prefix, ctxutilHost(c))
	content := fmt.Sprintf("installed_at=%s\nhost=%s\nport=%d\ndatabase=%s\n",
		time.Now().Format(time.RFC3339), host, port, dbName)
	if err := os.WriteFile(lock, []byte(content), 0o644); err != nil {
		response.Fail(c, "写入安装锁失败："+err.Error())
		return
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	response.Success(c, "安装成功", gin.H{"lock": lock, "imported": imported, "env": envPath})
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
