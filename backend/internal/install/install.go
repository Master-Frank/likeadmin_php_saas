package install

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"

	"github.com/gin-gonic/gin"
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
	if msg := EnvBlocking(); msg != "" {
		response.Fail(c, msg)
		return
	}
	p := httpx.Body(c)
	if msg := CheckParams(p); msg != "" {
		response.Fail(c, msg)
		return
	}

	host := firstNonEmpty(p, "host", "hostname", "db_host")
	if host == "" {
		host = "127.0.0.1"
	}
	port := httpx.BodyInt(c, "port")
	if port == 0 {
		port = httpx.BodyInt(c, "hostport")
	}
	if port == 0 {
		port = httpx.BodyInt(c, "db_port")
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

	lockPath := lock
	if lockPath == "" {
		lockPath = filepath.Join(config.C.App.PublicDir, "../config/install.lock")
	}
	envPath := filepath.Join(filepath.Dir(lockPath), "..", ".env")
	if httpx.BodyStr(c, "env_path") != "" {
		envPath = httpx.BodyStr(c, "env_path")
	}
	res, err := Apply(Options{
		Host: host, Port: port, User: user, Password: pass, Name: dbName, Prefix: prefix,
		ClearDB: clearDB, ImportTest: importTest, SkipSQL: httpx.BodyInt(c, "skip_sql") == 1,
		AdminUser: adminUser, AdminPassword: adminPass,
		PublicDir: config.C.App.PublicDir, LockPath: lockPath, EnvPath: envPath,
		GoConfigPath: httpx.BodyStr(c, "go_config_path"), HTTPHost: ctxutilHost(c),
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	// Empty go_config_path still updates in-memory config + default config.yaml.
	if httpx.BodyStr(c, "go_config_path") == "" {
		if err := WriteGoConfig("", host, dbName, user, pass, port, prefix, ctxutilHost(c), res.Salt); err != nil {
			response.Fail(c, "写入环境配置失败："+err.Error())
			return
		}
	}
	if err := bootstrap.ReconnectDB(); err != nil {
		response.Fail(c, "安装成功但数据库重连失败："+err.Error())
		return
	}
	tenantdb.Register(bootstrap.DB)
	response.Success(c, "安装成功", gin.H{"lock": res.Lock, "imported": res.Imported, "env": res.Env})
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

// setPHPSQLMode mirrors PHP installModel::connectDB / TenantCreatService::connectDB.
// Permission errors are ignored the same way PHP wraps this in an empty catch.
func setPHPSQLMode(db *gorm.DB) {
	if db == nil {
		return
	}
	_ = db.Exec("SET GLOBAL sql_mode='STRICT_TRANS_TABLES,ERROR_FOR_DIVISION_BY_ZERO,NO_AUTO_CREATE_USER,NO_ENGINE_SUBSTITUTION'").Error
}
