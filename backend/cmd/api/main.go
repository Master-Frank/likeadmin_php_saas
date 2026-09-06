package main

import (
	"log"
	"os"
	"path/filepath"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/router"
)

func main() {
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = filepath.Join(findConfigs(), "configs", "config.yaml")
	}
	if err := bootstrap.Init(cfg); err != nil {
		log.Fatalf("init: %v", err)
	}
	r := router.New()
	addr := config.C.App.Listen
	if addr == "" {
		addr = ":8080"
	}
	log.Printf("likeadmin-go listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}

func findConfigs() string {
	wd, _ := os.Getwd()
	for _, p := range []string{wd, filepath.Join(wd, "backend"), filepath.Join(wd, "..")} {
		if _, err := os.Stat(filepath.Join(p, "configs", "config.yaml")); err == nil {
			return p
		}
	}
	return wd
}
