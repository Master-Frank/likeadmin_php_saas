package main

import (
	"log"
	"os"
	"path/filepath"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
)

func main() {
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		wd, _ := os.Getwd()
		cfg = filepath.Join(wd, "configs", "config.yaml")
		if _, err := os.Stat(cfg); err != nil {
			cfg = filepath.Join(wd, "backend", "configs", "config.yaml")
		}
	}
	if err := bootstrap.Init(cfg); err != nil {
		log.Fatalf("init: %v", err)
	}
	runOnce()
}

func runOnce() {
	var rows []model.Crontab
	bootstrap.DB.Where("status = 1").Find(&rows)
	now := util.NowUnix()
	for _, item := range rows {
		if item.LastTime == nil {
			t := now
			bootstrap.DB.Model(&item).Update("last_time", t)
			continue
		}
		// Minimal runner: if last_time older than 60s, mark executed.
		if now-*item.LastTime >= 60 {
			start := time.Now()
			bootstrap.DB.Model(&item).Updates(map[string]any{
				"last_time": now,
				"error":     "",
				"time":      time.Since(start).String(),
			})
		}
	}
}
