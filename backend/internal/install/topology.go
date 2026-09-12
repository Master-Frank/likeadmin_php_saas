package install

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func topologyMode(p map[string]any, key, def string) string {
	v := strings.ToLower(strings.TrimSpace(pick(p, key)))
	if v == "" || v == "0" {
		return def
	}
	return v
}

// CheckTopology validates optional wizard deploy/db mode fields.
func CheckTopology(p map[string]any) string {
	deploy := topologyMode(p, "deploy_mode", "single")
	if deploy != "single" && deploy != "multi" {
		return "请选择部署模式"
	}
	dbm := topologyMode(p, "db_mode", "single")
	if dbm != "single" && dbm != "replica" {
		return "请选择数据库模式"
	}
	if deploy == "multi" && pick(p, "redis_host") == "" {
		return "多实例部署必须填写 Redis 地址"
	}
	if dbm == "replica" {
		if firstNonEmpty(p, "replica_host", "replica_hostname") == "" {
			return "请填写从库主机"
		}
	}
	return ""
}

func CheckRedis(host, password string, port, db int) error {
	if host == "" {
		return fmt.Errorf("Redis 地址不能为空")
	}
	if port == 0 {
		port = 6379
	}
	cli := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", host, port),
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	defer func() { _ = cli.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := cli.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("Redis 无法连接:%s", trimErr(err.Error()))
	}
	return nil
}

func CheckReplica(host, user, pass, name string, port int) error {
	if host == "" {
		return fmt.Errorf("从库主机不能为空")
	}
	if port == 0 {
		port = 3306
	}
	if err := CheckPort(host, port); err != nil {
		return fmt.Errorf("从库无法连接:%s", trimErr(err.Error()))
	}
	if name == "" {
		return nil
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=false&loc=Local",
		user, pass, host, port, name)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("从库无法连接:%s", trimErr(err.Error()))
	}
	var one int
	if err := db.Raw("SELECT 1").Scan(&one).Error; err != nil {
		return fmt.Errorf("从库无法连接:%s", trimErr(err.Error()))
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	return nil
}
