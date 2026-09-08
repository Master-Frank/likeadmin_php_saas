# likeadmin-SaaS Go 后端

渐进式替换 `server/` 下的 ThinkPHP 后端。接口前缀、JSON 信封、`token` Header、密码算法与 PHP 保持一致。

**迁移交接：** 完成范围、live 对拍、删 PHP 前检查清单见仓库根目录 [docs/php-to-go-status.md](../docs/php-to-go-status.md) 与 [AGENTS.md](../AGENTS.md)。

本机黄金对拍（直连 live `pair.sh` `failed=0`，2026-09-08）已绿。未覆盖路径不再默认回落 PHP。`server/` 仍保留静态资源与 `like.sql`，不要整树删除。

## 运行

```bash
cd backend
go mod tidy
export LIKEADMIN_CONFIG=$(pwd)/configs/config.yaml
go run ./cmd/api
```

默认监听 `:8080`。可用 `LIKEADMIN_LISTEN=:8080` 覆盖。

生产构建及初次安装：

```bash
make build
sudo make install PREFIX=/opt/likeadmin/backend
```

生产 systemd 单元会设置 `LIKEADMIN_DEBUG=false`、检查数据库账号具备
`CREATE`/`DROP`（分表租户及结构升级需要）。授权模板见
`deploy/mysql.production.example.sql`。

切流前门（API → Go，静态 / SPA 出自 `LIKEADMIN_PUBLIC`，默认 `server/public`）：

```bash
LIKEADMIN_STRANGLER=127.0.0.1:8090 \
LIKEADMIN_GO=http://127.0.0.1:8080 \
LIKEADMIN_PUBLIC=/workspace/server/public \
go run ./cmd/strangler
```

`LIKEADMIN_PHP_FALLBACK` 默认关闭。只有仍需临时代理未知 PHP 路径时才设为 `1`。

生产 Nginx（无 php-fpm）见 `deploy/nginx.production.conf`。本机切流校验用 `deploy/nginx.local.conf`（`:8091`）。
systemd 单元：`deploy/likeadmin-api.service`、`deploy/likeadmin-crontab.service`（把路径改成实际安装目录后 `systemctl enable --now`）。
在线升级若包含 `project/backend/`，会先在完整源码副本中构建新
`bin/api`/`bin/crontab`，构建失败不应用升级。启用
`likeadmin-upgrade-restart.path` + `.service` 后，升级成功会延迟重启两个
Go 服务；没有 `LIKEADMIN_UPGRADE_RESTART_COMMAND` 时拒绝在线应用 Go 后端包。

定时任务：

```bash
go run ./cmd/crontab          # 循环执行 la_dev_crontab
go run ./cmd/think            # 等价 php think，列出已迁命令
go run ./cmd/think help clear
go run ./cmd/think crontab    # 等价 php think crontab，只跑一轮
go run ./cmd/think query_refund
go run ./cmd/think run --port 8000
LIKEADMIN_ENABLE_PHP_SCAFFOLD=1 go run ./cmd/think make:controller tenantapi@Demo
```

`make:*`、`build`、`vendor:publish`、`service:discover` 会产生 PHP 文件，
默认关闭；只有兼容旧开发流程时显式设置
`LIKEADMIN_ENABLE_PHP_SCAFFOLD=1`。Go 运行时和生产部署不需要这些产物。

未知 `la_dev_crontab.command` 记「未定义的定时任务命令」，不再回落 `php think`。
仓库内 `make:*` / `vendor:publish` / `service:discover` / `build` 已迁；`think run` 起 Go HTTP（默认 `:8000`）。
第三方仓库命令写 `configs/think-commands.yaml`（绝对路径、禁止 php），或 `cron.Register`。

离线升级包（已下载的 zip，无需 mddai.cn）：

```bash
# ApplyLocal / file:// 与远程 link 走同一套解压+SQL+文件管道
go run ./cmd/think upgrade-local /path/to/package.zip

# 列表/授权走本地夹具（lists.json + verify.json），不打 mddai.cn
export LIKEADMIN_UPGRADE_FIXTURE=/path/to/upgrade-fixture
```

## 契约

- URL：`/{platformapi|tenantapi|api}/{controller}/{action}`，嵌套控制器用点号，如 `/platformapi/auth.admin/lists`
- 响应：`{code, show, msg, data}`
- 鉴权 Header：`token`
- 密码：`md5(salt + md5(password + salt))`，salt 为 `project.unique_identification`

验收清单见 [`tests/golden/README.md`](tests/golden/README.md)。
