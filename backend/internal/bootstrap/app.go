package bootstrap

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/dbindex"
	"likeadmin/backend/internal/metrics"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var (
	DB     *gorm.DB
	ReadDB *gorm.DB
	RDB    *redis.Client
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
	if err := initRedis(); err != nil {
		return err
	}
	if Installed() {
		dbindex.EnsurePerfIndexes(DB)
	}
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

// ReconnectRedis opens Redis after /install writes a new topology.
func ReconnectRedis() error {
	return initRedis()
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

func requireRedis() bool {
	return config.RequireRedisConfigured()
}

// Read returns the replica session when configured and healthy, otherwise the master.
func Read() *gorm.DB {
	if ReadDB == nil || ReadDB == DB {
		if ReadDB != nil {
			return ReadDB
		}
		return DB
	}
	if replicaHealthy() {
		return ReadDB
	}
	return DB
}

var replicaHealth struct {
	mu      sync.Mutex
	checked time.Time
	live    bool
}

func replicaHealthy() bool {
	replicaHealth.mu.Lock()
	defer replicaHealth.mu.Unlock()
	if time.Since(replicaHealth.checked) < 5*time.Second {
		return replicaHealth.live
	}
	replicaHealth.checked = time.Now()
	replicaHealth.live = pingDB(ReadDB)
	return replicaHealth.live
}

func pingDB(db *gorm.DB) bool {
	if db == nil || db.Config == nil {
		return false
	}
	sqlDB, err := db.DB()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout(config.C.Redis.DialTimeoutMs))
	defer cancel()
	return sqlDB.PingContext(ctx) == nil
}

func resetReplicaHealth() {
	replicaHealth.mu.Lock()
	replicaHealth.checked = time.Time{}
	replicaHealth.live = true
	replicaHealth.mu.Unlock()
}

func PingRedis() error {
	if RDB == nil {
		return fmt.Errorf("redis unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout(config.C.Redis.DialTimeoutMs))
	defer cancel()
	return RDB.Ping(ctx).Err()
}

// RequireRedis fails closed in production when LIKEADMIN_REQUIRE_REDIS=1.
func RequireRedis() error {
	if !requireRedis() || !Installed() {
		return nil
	}
	if RDB == nil {
		return fmt.Errorf("redis required but unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout(config.C.Redis.DialTimeoutMs))
	defer cancel()
	if err := RDB.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis required: %w", err)
	}
	return nil
}

func redisTimeout(ms int) time.Duration {
	if ms <= 0 {
		return 200 * time.Millisecond
	}
	return time.Duration(ms) * time.Millisecond
}

// RequestDB returns bootstrap.DB bound to the request context so SQL metrics attach.
func RequestDB(c *gin.Context) *gorm.DB {
	return requestSession(c, DB)
}

// RequestReadDB returns the replica (or master) bound to the request context.
func RequestReadDB(c *gin.Context) *gorm.DB {
	return requestSession(c, Read())
}

func requestSession(c *gin.Context, db *gorm.DB) *gorm.DB {
	if db == nil {
		return nil
	}
	if c != nil && c.Request != nil {
		return db.WithContext(c.Request.Context())
	}
	return db
}

func initDB() error {
	db, err := openGorm(config.C.Database)
	if err != nil {
		return err
	}
	metrics.Register(db)
	DB = db
	bindReadDB(db)
	return nil
}

func bindReadDB(master *gorm.DB) {
	ReadDB = master
	list := config.C.Database.ReplicaList()
	if master == nil || len(list) == 0 {
		return
	}
	rep := fillReplica(list[0], config.C.Database)
	db, err := openGorm(rep)
	if err != nil {
		log.Printf("replica unavailable, reads stay on master: %v", err)
		return
	}
	metrics.Register(db)
	ReadDB = db
}

func fillReplica(r, master config.DatabaseConfig) config.DatabaseConfig {
	if r.Hostport == 0 {
		r.Hostport = master.Hostport
	}
	if r.Database == "" {
		r.Database = master.Database
	}
	if r.Username == "" {
		r.Username = master.Username
	}
	if r.Password == "" {
		r.Password = master.Password
	}
	if r.Charset == "" {
		r.Charset = master.Charset
	}
	if r.Prefix == "" {
		r.Prefix = master.Prefix
	}
	if r.MaxOpenConns <= 0 {
		r.MaxOpenConns = master.MaxOpenConns
	}
	if r.MaxIdleConns <= 0 {
		r.MaxIdleConns = master.MaxIdleConns
	}
	if r.ConnMaxLifetime <= 0 {
		r.ConnMaxLifetime = master.ConnMaxLifetime
	}
	if r.ConnMaxIdleTime <= 0 {
		r.ConnMaxIdleTime = master.ConnMaxIdleTime
	}
	return r
}

func openGorm(c config.DatabaseConfig) (*gorm.DB, error) {
	if c.Charset == "" {
		c.Charset = "utf8mb4"
	}
	if c.Hostport == 0 {
		c.Hostport = 3306
	}
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
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(c.MaxOpenConns)
	sqlDB.SetMaxIdleConns(c.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(c.ConnMaxLifetime) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(c.ConnMaxIdleTime) * time.Second)
	return db, nil
}

func initRedis() error {
	c := config.C.Redis
	RDB = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", c.Host, c.Port),
		Password:     c.Password,
		DB:           c.DB,
		DialTimeout:  redisTimeout(c.DialTimeoutMs),
		ReadTimeout:  redisTimeout(c.ReadTimeoutMs),
		WriteTimeout: redisTimeout(c.WriteTimeoutMs),
	})
	ctx, cancel := context.WithTimeout(context.Background(), redisTimeout(c.DialTimeoutMs))
	defer cancel()
	if err := RDB.Ping(ctx).Err(); err != nil {
		if requireRedis() && Installed() {
			return fmt.Errorf("redis required: %w", err)
		}
		log.Printf("redis unavailable (%v), fallback to in-memory cache", err)
		_ = RDB.Close()
		RDB = nil
	}
	return nil
}

func RedisKey(k string) string {
	return config.C.Redis.Prefix + k
}
