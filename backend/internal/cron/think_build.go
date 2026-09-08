package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func runBuild(args []string) string {
	root := runtimeRoot()
	if root == "" {
		return "无法解析应用目录"
	}
	app := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			app = a
			break
		}
	}
	base := filepath.Join(root, "app")
	if err := os.MkdirAll(base, 0755); err != nil {
		return err.Error()
	}
	appPath := base
	ns := "app"
	if app != "" {
		appPath = filepath.Join(base, app)
		ns = "app\\" + app
		if err := os.MkdirAll(appPath, 0755); err != nil {
			return err.Error()
		}
	}
	if err := buildCommon(appPath); err != nil {
		return err.Error()
	}
	if err := buildHello(appPath, app, ns); err != nil {
		return err.Error()
	}
	for _, dir := range []string{"controller", "model", "view"} {
		if err := os.MkdirAll(filepath.Join(appPath, dir), 0755); err != nil {
			return err.Error()
		}
	}
	fmt.Println("Successed")
	return ""
}

func buildCommon(appPath string) error {
	common := filepath.Join(appPath, "common.php")
	if _, err := os.Stat(common); err != nil {
		if err := os.WriteFile(common, []byte("<?php\n// 这是系统自动生成的公共文件\n"), 0644); err != nil {
			return err
		}
	}
	for _, name := range []string{"event", "middleware", "common"} {
		path := filepath.Join(appPath, name+".php")
		if name == "common" {
			path = filepath.Join(appPath, "common.php")
			// common.php already written as the public file; Think also writes
			// event/middleware/common definition files. The third loop name
			// "common" would collide — PHP writes common.php once as the
			// public file, then event.php / middleware.php / common.php as
			// return arrays if missing. After the public write, common.php
			// exists so the array file is skipped. Match that.
			continue
		}
		if _, err := os.Stat(path); err == nil {
			continue
		}
		body := "<?php\n// 这是系统自动生成的" + name + "定义文件\nreturn [\n\n];\n"
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			return err
		}
	}
	return nil
}

func buildHello(appPath, app, ns string) error {
	filename := filepath.Join(appPath, "controller", "IndexController.php")
	if _, err := os.Stat(filename); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	body := `<?php
declare (strict_types = 1);

namespace ` + ns + `\controller;

class IndexController
{
    public function index()
    {
        return '您好！这是一个[` + app + `]示例应用';
    }
}
`
	return os.WriteFile(filename, []byte(body), 0644)
}
