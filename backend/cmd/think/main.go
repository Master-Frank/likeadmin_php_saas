// Command think is a PHP `php think <name>` drop-in for the commands
// likeadmin ships in-repo. Unknown names stay undefined so custom
// warehouse commands are not silently swallowed.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cron"
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
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: think <command> [params...]")
		os.Exit(1)
	}
	msg := cron.RunNamed(os.Args[1], os.Args[2:]...)
	if msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
