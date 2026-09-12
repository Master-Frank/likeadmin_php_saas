# PHP → Go 后端迁移状态

给后续维护者和 AI 的交接文档。产品前端（`platform/`、`tenant/`、`pc/`、`uniapp/`）仍是 Vue/uniapp；后端运行时走 Go（`backend/`）。ThinkPHP `server/` 已从工作树删除。

**更新日期：** 2026-09-12

## 一句话结论

Go 覆盖原 PHP 全部公开 HTTP 动作、Think CLI、系统 crontab、代码生成器运行时和安装向导。静态资源在仓库根 `public/`。SQL 只读 `backend/internal/sqlassets/`。默认测试与开发环境是 Go-only，不再启动 PHP。

## 仓库布局

| 路径 | 角色 |
|---|---|
| `backend/` | Go 后端（HTTP `cmd/api`、CLI `cmd/think`、定时 `cmd/crontab`、切流 `cmd/strangler`） |
| `public/` | SPA（`platform`/`admin`/`mobile`/`pc`）、`resource/`、`error/`、`uploads/` |
| `config/install.lock` | 安装锁（不入库） |
| `.env` `runtime/` `upgrade/` | 安装产物 / 导出 / 升级暂存（不入库） |
| `platform/` `tenant/` `pc/` `uniapp/` | 前端源码。发布脚本写入 `public/` |
| `backend/deploy/nginx.production.conf` | 生产切流：API / install / crontab → Go，`root` 为 `public/` |
| `backend/internal/router/coverage_test.go` | 前端路由硬编码白名单 |

## 删 `server/` 后如何对照 PHP

工作树和运行时都没有 PHP。排障时使用删树前的不可变锚点：

- annotated tag：`php-reference-final-20260912`（commit `23ba7183dd320e281ee142ac81089dbef34870eb`）
- 单文件：`git show php-reference-final-20260912:server/app/platformapi/logic/LoginLogic.php`
- 并排目录：`git worktree add /tmp/likeadmin-php-ref php-reference-final-20260912`
- `origin/develop` 只是上游 PHP 1.0.7 血缘，**不是** cutover 最终状态，不要用它替代该 tag。

偶发需要再跑 PHP 对拍时，只在该 worktree 里启动 PHP，并显式设置 `PHP=http://127.0.0.1:8000`。默认 `pair.sh` 的 `PHP` 等于 `GO`。

## 已完成（不要重做）

- **HTTP：** 原 PHP 公开控制器动作已在 Go。多出来的路由必须落在 Vue 共用 / 已注释 PHP 动作白名单（见 `coverage_test.go`）。
- **Think CLI：** `backend/cmd/think`。**禁止 `exec PHP`。** PHP `make:*` / `vendor:publish` / `build` 脚手架永久关闭。
- **crontab：** `query_refund`、`cancel_unpaid_orders`、`verification_orders`；启动时 `EnsureNativeJobs` 入库；独立 worker `cmd/crontab`。
- **代码生成器：** stub 在 `backend/internal/generator/stub/`。`generate_type=1` 写 Vue + 菜单 + `backend/internal/generated/*.go`。运行时 CRUD：`gencrud.Handle`。
- **安装向导：** `GET/POST /install`。lock 存在时文案与历史 PHP 一致。
- **前端跳转：** 未安装时跳 `/install`。
- **nginx：** `backend/deploy/nginx.production.conf` 的 `location /install` 已 `proxy_pass` Go；`root` 为 `/opt/likeadmin/public`。
- **SQL：** 只读 Go 内嵌 `backend/internal/sqlassets/{like,tenant,tenantData}.sql`。
- **在线升级：** 产品根是 `public/` 的父目录；解压暂存 `upgrade/`。旧 zip 前缀 `project/server/public/` 会映射到 `public/`。

Go **严于** 历史 PHP，不要为字节级一致回退：

- 租户删除/停用会清会话和分表
- 写接口用 GET 返回「请求方式错误」
- 分表租户 admin 走 `la_tenant_admin_{sn}`

允许差异：新签发 `token`、JSON 键顺序、工作台随机演示曲线。不允许：`code`/`show`/`msg` 语义、列表字段、空 `data` 形态、时间格式。

对拍中已对齐、不要再改回去：

- 非法 cron 表达式：先 `Fail()`「定时任务运行规则错误」
- 上传无文件：先 `ReceiveUpload`「未找到上传文件的信息」，再校验 cid
- 管理员密码 salt 必须是 `likeadmin`
- `delete_time` 用 SQL `NULL`，不要用 `0`

## 生产迁移（从 `server/public` 切到 `public/`）

现网数据不在 git 里。上线前必须备份并拷贝：

1. `server/public/uploads` → `public/uploads`
2. `server/.env` → 仓库根 `.env`
3. `server/config/install.lock` → `config/install.lock`
4. 把 nginx `root` 改成 `/opt/likeadmin/public`（或等价路径）
5. 确认 `LIKEADMIN_CONFIG` 里 `public_dir` / `install_lock` 指向新位置
6. 停 php-fpm；crontab 用 `cmd/crontab`

回滚：把上述文件拷回旧路径，并把 nginx `root` 改回旧值。

## 不要提交的文件

- `.env`、`config/install.lock`、`backend/internal/install/.env`
- 对拍生成物：`platform/src/api/pair_gencrud.ts`、`platform/src/views/pair_gencrud/`、`tenant/src/api/pair_tenant_crud.ts`、`tenant/src/views/pair_tenant_crud/`
- `public/uploads/*`（占位 `index.html` 除外）、`upgrade/`、`runtime/`

## 常用命令

```bash
cd backend
export LIKEADMIN_CONFIG=$(pwd)/configs/config.yaml
go run ./cmd/api              # :8080
go run ./cmd/crontab          # 循环执行 la_dev_crontab
go run ./cmd/think            # 等价历史 php think
go test ./internal/router -count=1

export GO=http://127.0.0.1:8080 TENANT_HOST=pair1.likeadmin.test
./backend/tests/golden/pair.sh
```

更细的模块表和对拍口径见 [`backend/tests/golden/README.md`](../backend/tests/golden/README.md)，Go 运行说明见 [`backend/README.md`](../backend/README.md)。

## 分支与 PR

后续功能请从 `main` 拉分支。不要从 PHP-only 的 `develop` 起新后端工作。PHP 对照只用 tag `php-reference-final-20260912`。

## 后续 AI 不要做的事

- 不要重做已迁 HTTP Logic / Think CLI / gencrud。
- 不要为与 PHP 字节级一致而回退分表清理、删除停用清理、写接口 GET 拒绝。
- 不要把 PHP 四步 layui 安装页做成 Go 克隆。
- 不要 `exec PHP` 跑 think 命令。
- 不要恢复 ThinkPHP `server/` 作为运行时依赖。
- 不要提交 `.env` / `install.lock` / pair 生成的 CRUD Vue。
