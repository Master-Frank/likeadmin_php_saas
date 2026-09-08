// Command think is a PHP `php think <name>` drop-in for the commands
// likeadmin ships in-repo. Unknown names stay undefined so custom
// warehouse commands are not silently swallowed.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/cron"
	"likeadmin/backend/internal/router"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/upgrade"
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
	tenantdb.Register(bootstrap.DB)
	// PHP Console::$defaultCommand = 'list' — bare `php think` lists commands.
	if len(os.Args) < 2 {
		os.Args = []string{os.Args[0], "list"}
	} else if os.Args[1] == "-h" || os.Args[1] == "--help" {
		os.Args[1] = "help"
	}
	if os.Args[1] == "run" {
		listen := cron.ParseRunArgs(os.Args[2:])
		if listen.Root != "" {
			config.C.App.PublicDir = listen.Root
		}
		fmt.Printf("ThinkPHP Development server is started On <http://%s:%s/>\n", listen.Host, listen.Port)
		fmt.Println("You can exit with `CTRL-C`")
		fmt.Printf("Document root is: %s\n", config.C.App.PublicDir)
		if err := router.New().Run(listen.Addr()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if os.Args[1] == "upgrade-local" {
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: think upgrade-local <zip>")
			os.Exit(1)
		}
		ver := ""
		if len(os.Args) >= 4 {
			ver = os.Args[3]
		}
		if err := upgrade.ApplyLocal(os.Args[2], ver); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	msg := cron.RunNamed(os.Args[1], os.Args[2:]...)
	if msg != "" {
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
