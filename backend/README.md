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

平台端入口校验 `project.http_host`：与浏览器地址栏主机不一致时会返回「平台端入口域名错误」。用 `http://127.0.0.1:8080/platform/` 访问时，该项应写成 `127.0.0.1:8080`；不限域名则置空。

PC 端入口是 `/pc/`。装修轮播等店铺链接沿用 uniapp 路径（如 `/pages/news/news`），Go 会 302 到对应 PC 页面（资讯中心 `/pc/information`），**不会**进 H5 `/mobile/`。H5 请直接访问 `/mobile/`。

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

安装向导（`GET/POST /install`）可选拓扑，默认仍是单实例 + 单数据库：

- **单实例**：不强制 Redis；导出在请求内完成，立刻 `ready`。
- **多实例**：必须能连 Redis；写入 `app.multi_instance`、`project.export_async`、`app.require_redis`。多台 Go 还需共享 `public_dir` 或事后在后台改 OSS。crontab 已有 MySQL `GET_LOCK`，可只跑一个 crontab 进程。
- **单库 / 主从**：主从只把日志列表、工作台计数等可延迟读打到 `database.replicas`；空配置读写都走主库。

观测：`LIKEADMIN_INSTANCE_ID`（默认 hostname）；`LIKEADMIN_METRICS=1` 时 `127.0.0.1:9090/metrics`（不要挂到公网 API 域）；`LIKEADMIN_PPROF=1` 时 `127.0.0.1:6060`。
可选 `app.cdn_domain` 给本地上传拼 CDN 前缀。`LIKEADMIN_EXPORT_ASYNC=1` 可在单实例也走导出队列。

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
- 导出：`export=1` 预览；`export=2` 返回 `{task_id,status,url?}`（`code=1`）。单实例默认同步 `ready`；多实例/异步为 `pending`，轮询 `GET /platformapi/download/export?task=`，下载仍用 `?file=`

验收清单见 [`tests/golden/README.md`](tests/golden/README.md)。
