package main

import (
	"log"
	"os"
	"path/filepath"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cron"
	"likeadmin/backend/internal/tenantdb"
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
	tenantdb.Register(bootstrap.DB)
	if os.Getenv("LIKEADMIN_CRON_ONCE") == "1" {
		cron.RunOnce()
		return
	}
	log.Printf("crontab worker started")
	cron.Loop(0)
}
