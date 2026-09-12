# AGENTS

likeadmin-SaaS：多租户管理后台。前端仍是 Vue/uniapp；**后端是 Go**（`backend/`）。仓库不再包含 ThinkPHP `server/` 树。

完整迁移状态、PHP 对照 tag、验收口径：

→ **[docs/php-to-go-status.md](docs/php-to-go-status.md)**

## 当前状态（2026-09-12）

- HTTP / Think CLI / 系统 crontab / 代码生成器 / 安装向导均是 Go。
- 静态资源、SPA、上传目录在仓库根 [`public/`](public/)；nginx `root` 指向 `public/`。
- SQL 只读 [`backend/internal/sqlassets/`](backend/internal/sqlassets/)。
- 路由守门：[`backend/internal/router/coverage_test.go`](backend/internal/router/coverage_test.go)。Go-only 契约脚本：`backend/tests/golden/pair.sh`（默认只打 `:8080`）。
- 最终 PHP 对照：annotated tag `php-reference-final-20260912`。单文件 `git show php-reference-final-20260912:server/app/...`；并排 `git worktree add /tmp/likeadmin-php-ref php-reference-final-20260912`。不要假设工作区还有 `server/`。
- 生产 service 强制 `LIKEADMIN_DEBUG=false` 并检查 DDL 权限；crontab 有跨进程 advisory lock 和唯一索引。
- 在线 Go 升级必须先构建二进制并配置 systemd 延迟重启。PHP 脚手架已永久关闭。

## 工作约定

- 接口前缀、JSON `{code,show,msg,data}`、`token` Header、密码算法与历史 PHP 一致。
- 管理员 salt 为 `likeadmin`（`project.unique_identification`）。`delete_time` 用 `NULL` 不是 `0`。
- 不要 `exec PHP`。不要提交 `.env`、`install.lock`、对拍生成的 `pair_*` Vue。
- 对拍口径与模块表：`backend/tests/golden/README.md`。Go 运行：`backend/README.md`。
