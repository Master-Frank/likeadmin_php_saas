# PHP → Go 后端迁移状态

给后续维护者和 AI 的交接文档。产品前端（`platform/`、`tenant/`、`pc/`、`uniapp/`）仍是 Vue/uniapp；后端运行时应走 Go（`backend/`），不要再扩展 ThinkPHP。

**更新日期：** 2026-09-08

## 一句话结论

Go 已覆盖 PHP 全部 **307/307** 个公开 HTTP 动作、Think CLI、系统 crontab、代码生成器运行时和安装向导。本机 live `./backend/tests/golden/pair.sh` 直连对拍 **`failed=0`**。仓库里的 `server/` 仍保留，作为静态资源、`like.sql` 和对照源，**还不能整树删除**。新环境安装走 Go `/install`，不要再跑 PHP 四步 layui 向导。

## 仓库布局

| 路径 | 角色 |
|---|---|
| `backend/` | Go 后端（HTTP `cmd/api`、CLI `cmd/think`、定时 `cmd/crontab`、切流 `cmd/strangler`） |
| `server/` | 原 ThinkPHP 后端 + `public/` 静态资源 + `public/install/db/like.sql`。对照用，生产不要再扩业务 |
| `platform/` `tenant/` `pc/` `uniapp/` | 前端，未改技术栈。源码里未安装跳转已改为 `/install` |
| `backend/deploy/nginx.production.conf` | 生产切流：API / install / crontab → Go，可停 php-fpm |
| `backend/internal/router/php_module_test.go` | 307 动作 + Logic/Lists/Validate/中间件/服务/缓存/命令守门 |

默认远程分支历史上是 **`develop`**（无 `main`）。2026-09-08 起用 **`main`** 收纳下面两个 PR 的提交。

## 已完成（不要重做）

- **HTTP：** PHP 公开控制器动作 307/307。Go 多出来的路由必须落在 Vue 共用 / 已注释 PHP 动作白名单。
- **Think CLI：** `backend/cmd/think`（`make:*`、`vendor:publish`、`service:discover`、`build`、`run`、`list`、`crontab` 等）。**禁止 `exec PHP`。**
- **crontab：** `query_refund`、`cancel_unpaid_orders`、`verification_orders`；启动时 `EnsureNativeJobs` 入库；独立 worker `cmd/crontab`。未知 `la_dev_crontab.command` 记「未定义的定时任务命令」，不回落 `php think`。
- **代码生成器：** stub 在 `backend/internal/generator/stub/`。`generate_type=1` 写 Vue + 菜单 + `backend/internal/generated/*.go`，**不再写** `server/app` PHP。运行时 CRUD：`gencrud.Handle`。
- **安装向导：** `GET/POST /install`、`GET /install/env`、`GET /install/check`、`Any /install/status`、`GET /install/install.php` → `install.Wizard`。lock 存在时文案与 PHP 一致：「可能已经安装过本系统了…」。
- **前端跳转：** `platform/src/utils/request/index.ts`、`tenant/src/utils/request/index.ts`、`pc/utils/http/index.ts` → `window.location.replace('/install')`。
- **PHP `index.php`：** 未安装时 302 到 `/install`（不再跳 `/install/install.php`）。
- **nginx：** `backend/deploy/nginx.production.conf` 的 `location /install` 已 `proxy_pass` Go。
- **`like.sql`：** 仍从 `server/public/install/db/like.sql` 读取。

刻意不迁（无路由或无控制器调用）：

- `LoginLogic::silentLogin`
- AliPay `transfer` / `transferQuery`
- `api/pay/notifyApp` 为 Go 多注册（与 `notifyMnp`/`notifyOa` 同处理器）

Go **严于** PHP，不要为字节级一致回退：

- 租户删除/停用会清会话和分表
- 写接口用 GET 返回「请求方式错误」
- 分表租户 admin 走 `la_tenant_admin_{sn}`

允许差异：新签发 `token`、JSON 键顺序、工作台随机演示曲线。不允许：`code`/`show`/`msg` 语义、列表字段、空 `data` 形态、时间格式。

对拍中已对齐、不要再改回去：

- 非法 cron 表达式：先 `Fail()`「定时任务运行规则错误」（与 PHP Validate），不要塞进 `data()`
- 上传无文件：先 `ReceiveUpload`「未找到上传文件的信息」，再校验 cid
- 管理员密码 salt 必须是 `likeadmin`（`backend/configs/config.yaml` 的 `project.unique_identification`）
- `delete_time` 用 SQL `NULL`，不要用 `0`

## Live 对拍（2026-09-08，本环境）

- PHP `:8000`（`php -S` + `server/public/router.php`）、Go `:8080`、MySQL `localhost_likeadmin`、Redis。
- 平台 admin / likeadmin；租户 pair1（`tactics=0`）/ pair2（`tactics=1`），Host `pair1.likeadmin.test` / `pair2.likeadmin.test`。
- **`./backend/tests/golden/pair.sh` live：`failed=0`（直连 `:8080`）。**
- 安装：有 lock 时 Go `/install` 与 `/install/install.php` 同文案；摘 lock 后是 Go 单页（「likeadmin 安装」「开始安装」），不含 layui。PHP `:8000/install/install.php` 仍是旧向导，生产 nginx 不会执行它。
- **本轮没有**再跑 Nginx `:8091` / strangler `:8090`，也没有 live 跑 `pair-gap.sh` / `pair-generator-zip.sh`。这两个脚本仍被 `TestPairScriptsMentionPHPActions` 守门，必须点名全部 307 个动作。
- 无真实微信/支付宝/短信凭证，对拍只覆盖失败/校验语义。

`pair.sh` 坑：

- `mysqlq` 需 `|| true`，避免重复键把整场对拍打挂
- crontab SQL 里 **`system` 列必须写成 \`system\`**（bash 双引号里反引号会被命令替换）

## 删 PHP 后端前要做的事

不必再从零扫 307 模块。删 `server/app` 等 PHP 应用树之前：

1. **生产切流：** 用 `backend/deploy/nginx.production.conf`（或等价配置）把 `/platformapi` `/tenantapi` `/api` `/install` `/crontab` 指到 Go，**停 php-fpm**。确认 `LIKEADMIN_PHP_FALLBACK` 关闭。
2. **保留安装 SQL：** 留下 `server/public/install/db/like.sql`（以及安装所需的 `public/` 静态），或改成 Go 内嵌后再删。
3. **真实凭证：** 有微信/支付宝/短信凭证后再验成功下单、退款、公众号菜单发布等路径。对拍目前只覆盖失败/校验。
4. **前端主路径：** 人工点一遍平台 / 租户 / PC 主流程。已构建的 JS 若仍写 `install.php`，Go 有别名，最好重编前端。
5. **crontab：** 用 `cmd/crontab` 或 `backend/deploy/likeadmin-crontab.service`，不要 `php think crontab`。

删树时还要一并处理：PHP vendor、Think 入口、仅对照用的 `index.php` 路由。静态上传目录、装修资源、前端 dist 若仍放在 `server/public`，不要误删。

## 不要提交的文件

- `backend/internal/install/.env`、`server/.env`、`install.lock`
- 对拍生成物：`platform/src/api/pair_gencrud.ts`、`platform/src/views/pair_gencrud/`、`tenant/src/api/pair_tenant_crud.ts`、`tenant/src/views/pair_tenant_crud/`、`server/public/upgrade/`、`server/upgrade/`

## 常用命令

```bash
cd backend
export LIKEADMIN_CONFIG=$(pwd)/configs/config.yaml
go run ./cmd/api              # :8080
go run ./cmd/crontab          # 循环执行 la_dev_crontab
go run ./cmd/think            # 等价 php think
go test ./internal/router -count=1   # 307 守门

export PHP=http://127.0.0.1:8000 GO=http://127.0.0.1:8080 TENANT_HOST=pair1.likeadmin.test
./backend/tests/golden/pair.sh
```

更细的模块表和对拍口径见 [`backend/tests/golden/README.md`](../backend/tests/golden/README.md)，Go 运行说明见 [`backend/README.md`](../backend/README.md)。

## 分支与 PR

| 项 | 说明 |
|---|---|
| 原默认分支 | `develop`（上游 likeadmin-SaaS PHP 基线） |
| PR [#1](https://github.com/Master-Frank/likeadmin_php_saas/pull/1) | `cursor/php-to-go-backend-9da4` → 原 `develop`：Go 后端主体 |
| PR [#2](https://github.com/Master-Frank/likeadmin_php_saas/pull/2) | `cursor/php-to-go-continue-25cd` → PR#1：crontab、生成器、安装切流、live 对拍修复 |
| `main` | 由 `develop` 依次合并上述两个 PR 的 tip（PR#2 包含 PR#1） |

后续功能请从 `main` 拉分支，不要从 PHP-only 的 `develop` 起新后端工作。

## 后续 AI 不要做的事

- 不要重做已迁 HTTP Logic / Think CLI / gencrud / 307 守门。
- 不要为与 PHP 字节级一致而回退分表清理、删除停用清理、写接口 GET 拒绝。
- 不要把 PHP 四步 layui 安装页做成 Go 克隆；现有单页向导即可。
- 不要 `exec PHP` 跑 think 命令。
- 不要提交 `.env` / `install.lock` / pair 生成的 CRUD Vue。
