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
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=false&loc=Local",
		user, pass, host, port, dbName)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		response.Fail(c, "数据库连接失败："+err.Error())
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
	if lock == "" {
		lock = filepath.Join(config.C.App.PublicDir, "../config/install.lock")
	}
	if err := os.MkdirAll(filepath.Dir(lock), 0o755); err != nil {
		response.Fail(c, err.Error())
		return
	}
	content := fmt.Sprintf("installed_at=%s\nhost=%s\nport=%d\ndatabase=%s\n",
		time.Now().Format(time.RFC3339), host, port, dbName)
	if err := os.WriteFile(lock, []byte(content), 0o644); err != nil {
		response.Fail(c, "写入安装锁失败："+err.Error())
		return
	}
	response.Success(c, "安装成功", gin.H{"lock": lock})
}

func CheckPort(host string, port int) error {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return err
	}
	return conn.Close()
}
