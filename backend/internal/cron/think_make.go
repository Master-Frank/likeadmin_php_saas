package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type makeKind struct {
	typ       string
	layer     string
	stub      string
	ctrlSuf   bool
	cmdName   bool
	ctrlFlags bool
}

var makeKinds = map[string]makeKind{
	"controller": {typ: "Controller", layer: "controller", stub: "controller", ctrlSuf: true, ctrlFlags: true},
	"model":      {typ: "Model", layer: "model", stub: "model"},
	"validate":   {typ: "Validate", layer: "validate", stub: "validate"},
	"middleware": {typ: "Middleware", layer: "middleware", stub: "middleware"},
	"event":      {typ: "Event", layer: "event", stub: "event"},
	"listener":   {typ: "Listener", layer: "listener", stub: "listener"},
	"subscribe":  {typ: "Subscribe", layer: "subscribe", stub: "subscribe"},
	"service":    {typ: "Service", layer: "service", stub: "service"},
	"command":    {typ: "Command", layer: "command", stub: "command", cmdName: true},
}

func makeRunner(kind string) CommandFunc {
	return func(args []string) string {
		return runMake(kind, args)
	}
}

func runMake(kind string, args []string) string {
	mk, ok := makeKinds[kind]
	if !ok {
		return fmt.Sprintf("未定义的命令: make:%s", kind)
	}
	flags, positionals := parseMakeArgs(args)
	if len(positionals) == 0 {
		return `Not enough arguments (missing: "name").`
	}
	name := strings.TrimSpace(positionals[0])
	if name == "" {
		return `Not enough arguments (missing: "name").`
	}
	className := thinkClassName(name, mk.layer, mk.ctrlSuf)
	pathName := thinkPathName(className)
	if pathName == "" {
		return "无法解析应用目录"
	}
	if _, err := os.Stat(pathName); err == nil {
		msg := mk.typ + ":" + className + " already exists!"
		fmt.Println(msg)
		return msg
	}
	stubName := mk.stub
	if mk.ctrlFlags {
		switch {
		case flags["api"]:
			stubName = "controller.api"
		case flags["plain"]:
			stubName = "controller.plain"
		}
	}
	commandName := ""
	if mk.cmdName {
		if len(positionals) > 1 && strings.TrimSpace(positionals[1]) != "" {
			commandName = strings.TrimSpace(positionals[1])
		} else {
			commandName = strings.ToLower(filepath.Base(strings.ReplaceAll(className, "\\", "/")))
		}
	}
	body := buildMakeClass(stubName, className, commandName)
	if err := os.MkdirAll(filepath.Dir(pathName), 0755); err != nil {
		return err.Error()
	}
	if err := os.WriteFile(pathName, []byte(body), 0644); err != nil {
		return err.Error()
	}
	fmt.Println(mk.typ + ":" + className + " created successfully.")
	return ""
}

func parseMakeArgs(args []string) (map[string]bool, []string) {
	flags := map[string]bool{}
	var pos []string
	for _, a := range args {
		switch a {
		case "--api":
			flags["api"] = true
		case "--plain":
			flags["plain"] = true
		default:
			if !strings.HasPrefix(a, "-") {
				pos = append(pos, a)
			}
		}
	}
	return flags, pos
}

func thinkClassName(name, layer string, controllerSuffix bool) string {
	name = strings.TrimSpace(name)
	name = strings.ReplaceAll(name, "/", "\\")
	if strings.Contains(name, "\\") && (strings.HasPrefix(name, "app\\") || strings.HasPrefix(name, "\\")) {
		if controllerSuffix && !strings.HasSuffix(name, "Controller") {
			return name + "Controller"
		}
		return name
	}
	app := ""
	if i := strings.Index(name, "@"); i >= 0 {
		app = name[:i]
		name = name[i+1:]
		name = strings.ReplaceAll(name, "/", "\\")
	}
	ns := "app"
	if app != "" {
		ns += "\\" + app
	}
	if layer != "" {
		ns += "\\" + layer
	}
	class := ns + "\\" + name
	if controllerSuffix && !strings.HasSuffix(class, "Controller") {
		class += "Controller"
	}
	return class
}

func thinkPathName(className string) string {
	root := runtimeRoot()
	if root == "" {
		return ""
	}
	name := className
	if strings.HasPrefix(name, "app\\") {
		name = name[4:]
	}
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimLeft(name, "/")
	return filepath.Join(root, "app", name+".php")
}

func buildMakeClass(stub, className, commandName string) string {
	ns := ""
	class := className
	if i := strings.LastIndex(className, "\\"); i >= 0 {
		ns = className[:i]
		class = className[i+1:]
	}
	body := makeStubs[stub]
	if body == "" {
		body = makeStubs["plain"]
	}
	repl := []string{
		"{%namespace%}", ns,
		"{%className%}", class,
		"{%actionSuffix%}", "",
		"{%app_namespace%}", "app",
		"{%commandName%}", commandName,
	}
	out := body
	for i := 0; i < len(repl); i += 2 {
		out = strings.ReplaceAll(out, repl[i], repl[i+1])
	}
	return out
}

var makeStubs = map[string]string{
	"controller": `<?php
declare (strict_types = 1);

namespace {%namespace%};

use think\Request;

class {%className%}
{
    /**
     * 显示资源列表
     *
     * @return \think\Response
     */
    public function index{%actionSuffix%}()
    {
        //
    }

    /**
     * 显示创建资源表单页.
     *
     * @return \think\Response
     */
    public function create{%actionSuffix%}()
    {
        //
    }

    /**
     * 保存新建的资源
     *
     * @param  \think\Request  $request
     * @return \think\Response
     */
    public function save{%actionSuffix%}(Request $request)
    {
        //
    }

    /**
     * 显示指定的资源
     *
     * @param  int  $id
     * @return \think\Response
     */
    public function read{%actionSuffix%}($id)
    {
        //
    }

    /**
     * 显示编辑资源表单页.
     *
     * @param  int  $id
     * @return \think\Response
     */
    public function edit{%actionSuffix%}($id)
    {
        //
    }

    /**
     * 保存更新的资源
     *
     * @param  \think\Request  $request
     * @param  int  $id
     * @return \think\Response
     */
    public function update{%actionSuffix%}(Request $request, $id)
    {
        //
    }

    /**
     * 删除指定资源
     *
     * @param  int  $id
     * @return \think\Response
     */
    public function delete{%actionSuffix%}($id)
    {
        //
    }
}
`,
	"controller.api": `<?php
declare (strict_types = 1);

namespace {%namespace%};

use think\Request;

class {%className%}
{
    /**
     * 显示资源列表
     *
     * @return \think\Response
     */
    public function index{%actionSuffix%}()
    {
        //
    }

    /**
     * 保存新建的资源
     *
     * @param  \think\Request  $request
     * @return \think\Response
     */
    public function save{%actionSuffix%}(Request $request)
    {
        //
    }

    /**
     * 显示指定的资源
     *
     * @param  int  $id
     * @return \think\Response
     */
    public function read{%actionSuffix%}($id)
    {
        //
    }

    /**
     * 保存更新的资源
     *
     * @param  \think\Request  $request
     * @param  int  $id
     * @return \think\Response
     */
    public function update{%actionSuffix%}(Request $request, $id)
    {
        //
    }

    /**
     * 删除指定资源
     *
     * @param  int  $id
     * @return \think\Response
     */
    public function delete{%actionSuffix%}($id)
    {
        //
    }
}
`,
	"controller.plain": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%}
{
    //
}
`,
	"model": `<?php
declare (strict_types = 1);

namespace {%namespace%};

use think\Model;

/**
 * @mixin \think\Model
 */
class {%className%} extends Model
{
    //
}
`,
	"validate": `<?php
declare (strict_types = 1);

namespace {%namespace%};

use think\Validate;

class {%className%} extends Validate
{
    /**
     * 定义验证规则
     * 格式：'字段名' =>  ['规则1','规则2'...]
     *
     * @var array
     */
    protected $rule = [];

    /**
     * 定义错误信息
     * 格式：'字段名.规则名' =>  '错误信息'
     *
     * @var array
     */
    protected $message = [];
}
`,
	"middleware": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%}
{
    /**
     * 处理请求
     *
     * @param \think\Request $request
     * @param \Closure       $next
     * @return Response
     */
    public function handle($request, \Closure $next)
    {
        //
    }
}
`,
	"event": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%}
{
}
`,
	"listener": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%}
{
    /**
     * 事件监听处理
     *
     * @return mixed
     */
    public function handle($event)
    {
        //
    }
}
`,
	"subscribe": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%}
{
}
`,
	"service": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%} extends \think\Service
{
    /**
     * 注册服务
     *
     * @return mixed
     */
    public function register()
    {
    	//
    }

    /**
     * 执行服务
     *
     * @return mixed
     */
    public function boot()
    {
        //
    }
}
`,
	"command": `<?php
declare (strict_types = 1);

namespace {%namespace%};

use think\console\Command;
use think\console\Input;
use think\console\input\Argument;
use think\console\input\Option;
use think\console\Output;

class {%className%} extends Command
{
    protected function configure()
    {
        // 指令配置
        $this->setName('{%commandName%}')
            ->setDescription('the {%commandName%} command');
    }

    protected function execute(Input $input, Output $output)
    {
        // 指令输出
        $output->writeln('{%commandName%}');
    }
}
`,
	"plain": `<?php
declare (strict_types = 1);

namespace {%namespace%};

class {%className%}
{
}
`,
}
