package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"likeadmin/backend/internal/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var (
	DB  *gorm.DB
	RDB *redis.Client
)

func Init(cfgPath string) error {
	if err := config.Load(cfgPath); err != nil {
		return err
	}
	if loc, err := time.LoadLocation(config.C.App.Timezone); err == nil {
		time.Local = loc
	}
	if err := initDB(); err != nil {
		if Installed() {
			return err
		}
		log.Printf("database unavailable before install: %v", err)
		DB = nil
	}
	initRedis()
	return nil
}

// Installed is true when PHP/Go install.lock exists.
func Installed() bool {
	lock := config.C.App.InstallLock
	if lock == "" {
		return false
	}
	_, err := os.Stat(lock)
	return err == nil
}

// ReconnectDB opens the DB after a successful /install (config.C already updated).
func ReconnectDB() error {
	return initDB()
}

var ddlIdent = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// CheckDDLPrivileges verifies the application account can provision sharded
// tenants and apply schema upgrades. It creates and removes one uniquely named
// empty table in the configured database.
func CheckDDLPrivileges() error {
	if DB == nil {
		return fmt.Errorf("database unavailable")
	}
	name := config.Prefix() + "go_ddl_probe_" + fmt.Sprint(time.Now().UnixNano())
	name = ddlIdent.ReplaceAllString(name, "_")
	if err := DB.Exec("CREATE TABLE `" + name + "` (`id` int NOT NULL PRIMARY KEY)").Error; err != nil {
		return fmt.Errorf("CREATE TABLE: %w", err)
	}
	if err := DB.Exec("DROP TABLE `" + name + "`").Error; err != nil {
		return fmt.Errorf("DROP TABLE: %w", err)
	}
	return nil
}

func RequireDDLPrivileges() error {
	if os.Getenv("LIKEADMIN_REQUIRE_DDL") == "0" || !Installed() {
		return nil
	}
	return CheckDDLPrivileges()
}

func initDB() error {
	c := config.C.Database
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=false&loc=Local",
		c.Username, c.Password, c.Hostname, c.Hostport, c.Database, c.Charset)
	level := logger.Warn
	if config.C.App.Debug {
		level = logger.Info
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(level),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(50)
	DB = db
	return nil
}

func initRedis() {
	c := config.C.Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Password: c.Password,
		DB:       c.DB,
	})
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		log.Printf("redis unavailable (%v), fallback to memory-less cache via DB only", err)
	}
}

func RedisKey(k string) string {
	return config.C.Redis.Prefix + k
}
