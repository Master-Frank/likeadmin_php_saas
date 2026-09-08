# AGENTS

likeadmin-SaaS：多租户管理后台。前端仍是 Vue/uniapp；**后端已迁到 Go**（`backend/`）。PHP `server/` 仅作静态资源、`like.sql` 和对照源，不要再往 ThinkPHP 加业务。

完整迁移状态、live 对拍结果、删 PHP 前检查清单：

→ **[docs/php-to-go-status.md](docs/php-to-go-status.md)**

## 当前状态（2026-09-08）

- PHP 公开 HTTP 动作 **307/307** 已在 Go，守门：`backend/internal/router/php_module_test.go`。
- Think CLI、系统 crontab、代码生成器（写 Vue + `backend/internal/generated`，不写 PHP）、安装向导（`/install`）均已是 Go。
- live `./backend/tests/golden/pair.sh` 直连 `:8080`：**`failed=0`**。
- like.sql / 租户 SQL 已内嵌到 `backend/internal/sqlassets/`；磁盘文件仍优先。
- 生产 service 强制 `LIKEADMIN_DEBUG=false` 并检查 DDL 权限；crontab 有跨进程 advisory lock 和唯一索引。
- 在线 Go 升级必须先构建二进制并配置 systemd 延迟重启；旧 PHP 脚手架默认关闭。
- **还不能删整个 `server/`**：静态资源、上传、升级包目录仍在 `server/public`。先按文档切 nginx、用真实凭证验支付/短信。

## 工作约定

- 接口前缀、JSON `{code,show,msg,data}`、`token` Header、密码算法与 PHP 一致。
- 管理员 salt 为 `likeadmin`（`project.unique_identification`）。`delete_time` 用 `NULL` 不是 `0`。
- 不要 `exec PHP`。不要提交 `.env`、`install.lock`、对拍生成的 `pair_*` Vue。
- 对拍口径与模块表：`backend/tests/golden/README.md`。Go 运行：`backend/README.md`。
