# AGENTS

likeadmin-SaaS：多租户管理后台。前端仍是 Vue/uniapp；**后端是 Go**（`backend/`）。仓库不再包含 ThinkPHP `server/` 树。

完整迁移状态、PHP 对照 tag、验收口径：

→ **[docs/php-to-go-status.md](docs/php-to-go-status.md)**

## 当前状态（2026-09-12）

- HTTP / Think CLI / 系统 crontab / 代码生成器 / 安装向导均是 Go。
- 静态资源、SPA、上传目录在仓库根 [`public/`](public/)；nginx `root` 指向 `public/`。
- SQL 只读 [`backend/internal/sqlassets/`](backend/internal/sqlassets/)。
- 路由守门：[`backend/internal/router/coverage_test.go`](backend/internal/router/coverage_test.go)。Go-only 契约脚本：`backend/tests/golden/pair.sh`（默认只打 `:8080`）。
- 最终 PHP 对照：annotated tag `php-reference-final-20260912`（commit `23ba7183dd320e281ee142ac81089dbef34870eb`）。工作区没有 `server/`，用下面「如何读 PHP 对照源码」。
- 生产 service 强制 `LIKEADMIN_DEBUG=false` 并检查 DDL 权限；crontab 有跨进程 advisory lock 和唯一索引。
- 在线 Go 升级必须先构建二进制并配置 systemd 延迟重启。PHP 脚手架已永久关闭。

## 如何读 PHP 对照源码

工作树和运行时都没有 ThinkPHP。**不要**把 `server/` 检回当前分支，也**不要**用 `origin/develop`（那是上游 PHP 1.0.7，不是 cutover 终态）。只读 annotated tag `php-reference-final-20260912`。

本地没有该 tag 时先取：

```bash
git fetch origin tag php-reference-final-20260912
```

**单文件 / 少量文件（首选）：**

```bash
git show php-reference-final-20260912:server/app/platformapi/logic/LoginLogic.php
git show php-reference-final-20260912:server/app/tenantapi/controller/LoginController.php
git show php-reference-final-20260912:server/app/api/logic/UserLogic.php
```

**按路径列文件、按关键字搜（不 checkout）：**

```bash
git ls-tree -r --name-only php-reference-final-20260912 -- server/app/platformapi/logic
git grep -n 'create_password' php-reference-final-20260912 -- server/app
git grep -n 'function account' php-reference-final-20260912 -- 'server/app/**/Login*.php'
```

**需要并排翻很多文件时再 worktree**（只读，用完删）：

```bash
git worktree add --detach /tmp/likeadmin-php-ref php-reference-final-20260912
# 源码在 /tmp/likeadmin-php-ref/server/app/...
git worktree remove /tmp/likeadmin-php-ref
```

路径对照（Go → tag 内 PHP）：

| Go | PHP（tag 内） |
|---|---|
| `backend/internal/platformapi/` | `server/app/platformapi/`（`controller` / `logic` / `lists` / `validate`） |
| `backend/internal/tenantapi/` | `server/app/tenantapi/` |
| `backend/internal/openapi/` | `server/app/api/` |
| `backend/internal/install/` | `server/public/install/`、安装相关 PHP |
| `backend/internal/cron/`、`cmd/think` | `server/app/common/command/` |
| `backend/internal/sqlassets/` | 历史 `server/public/install/db/`、`server/app/platformapi/db/`（现只读 embed） |

ThinkPHP 动作 `FooController::bar` 对应 HTTP `/platformapi/foo/bar`、`/tenantapi/foo/bar`、`/api/foo/bar`。默认不要启动 PHP、不要 `exec PHP`。只有人工双跑对拍时才在上述 worktree 里起 PHP，并显式 `PHP=http://127.0.0.1:8000`；`pair.sh` 默认 `PHP` 等于 `GO`。

## 工作约定

- 接口前缀、JSON `{code,show,msg,data}`、`token` Header、密码算法与历史 PHP 一致。
- 管理员 salt 为 `likeadmin`（`project.unique_identification`）。`delete_time` 用 `NULL` 不是 `0`。
- 不要 `exec PHP`。不要提交 `.env`、`install.lock`、对拍生成的 `pair_*` Vue。
- 对拍口径与模块表：`backend/tests/golden/README.md`。Go 运行：`backend/README.md`。
