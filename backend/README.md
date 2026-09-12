# likeadmin-SaaS Go 后端

接口前缀、JSON 信封、`token` Header、密码算法与历史 PHP 保持一致。

**迁移交接：** 完成范围、PHP 对照 tag、验收口径见仓库根目录 [docs/php-to-go-status.md](../docs/php-to-go-status.md) 与 [AGENTS.md](../AGENTS.md)。

默认 `pair.sh` 只打 Go `:8080`。静态资源在仓库根 `public/`。PHP 对照用 tag `php-reference-final-20260912`。

## 运行

```bash
cd backend
go mod tidy
export LIKEADMIN_CONFIG=$(pwd)/configs/config.yaml
go run ./cmd/api
```

默认监听 `:8080`。可用 `LIKEADMIN_LISTEN=:8080` 覆盖。生产 systemd 单元默认 `127.0.0.1:8080`，只信任来自本机/`LIKEADMIN_TRUSTED_PROXIES` 的 `X-Real-IP`。

生产索引请用 `bin/think ensure-indexes`（或 `LIKEADMIN_ENSURE_INDEXES=1`）显式创建，HTTP 启动默认不再串行 `CREATE INDEX`。`LIKEADMIN_REQUIRE_INDEXES=1` 可在缺索引时拒绝启动；`bin/think explain-indexes` 输出首批查询形状的 `EXPLAIN`，仅在明确设置 `LIKEADMIN_EXPLAIN_ANALYZE=1` 时执行 `EXPLAIN ANALYZE`。

2 核 2GB 且 MySQL 同机时，不要用满默认 `max_open_conns=50`。建议 `LIKEADMIN_DB_MAX_OPEN=15`、`LIKEADMIN_DB_MAX_IDLE=5`，并开启 `LIKEADMIN_EXPORT_ASYNC=1`。规划并发见仓库根目录 [performance.md](../performance.md) 第 2.8 节（规划口径，不是实测 QPS）。

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

切流前门（API → Go，静态 / SPA 出自 `LIKEADMIN_PUBLIC`，默认仓库根 `public/`）：

```bash
LIKEADMIN_STRANGLER=127.0.0.1:8090 \
LIKEADMIN_GO=http://127.0.0.1:8080 \
LIKEADMIN_PUBLIC=/workspace/public \
go run ./cmd/strangler
```

`LIKEADMIN_PHP_FALLBACK` 默认关闭。不要再把它打开去代理 PHP。

生产 Nginx（无 php-fpm）见 `deploy/nginx.production.conf`。本机切流校验用 `deploy/nginx.local.conf`（`:8091`）。
systemd 单元：`deploy/likeadmin-api.service`、`deploy/likeadmin-crontab.service`（把路径改成实际安装目录后 `systemctl enable --now`）。

安装向导（`GET/POST /install`）可选拓扑，默认仍是单实例 + 单数据库：

- **单实例**：不强制 Redis；导出在请求内完成，立刻 `ready`。
- **多实例**：必须能连 Redis；写入 `app.multi_instance`、`project.export_async`、`app.require_redis`。多台 Go 的导出目录还需通过 `LIKEADMIN_EXPORT_DIR` 指向同一私有共享挂载；公开上传使用共享 `public_dir` 或后台 OSS。crontab 已有 MySQL `GET_LOCK`，可只跑一个 crontab 进程。
- **单库 / 主从**：主从只把日志列表、工作台计数等可延迟读打到 `database.replicas`；空配置读写都走主库。`LIKEADMIN_REPLICA_MAX_LAG` 设置允许的复制延迟秒数（默认 30），`LIKEADMIN_REPLICA_HEALTH_TIMEOUT_MS` 设置后台探测超时（默认 1000ms）。无法读取复制延迟时默认回落主库；托管只读端点确实不提供 lag 时可显式设置 `LIKEADMIN_REPLICA_ALLOW_UNKNOWN_LAG=1`。连接类查询错误会将当前读回放到主库。

观测：`LIKEADMIN_INSTANCE_ID`（默认 hostname）；`LIKEADMIN_METRICS=1` 时 `127.0.0.1:9090/metrics`（不要挂到公网 API 域）；`LIKEADMIN_PPROF=1` 时 `127.0.0.1:6060`。
可选 `app.cdn_domain` 给本地上传拼 CDN 前缀。`LIKEADMIN_EXPORT_ASYNC=1` 可在单实例也让平台/租户后台导出走队列；C 端导出保留请求内用户上下文并同步完成。

在线升级若包含 `project/backend/`，会先在完整源码副本中构建新
`bin/api`/`bin/crontab`，构建失败不应用升级。启用
`likeadmin-upgrade-restart.path` + `.service` 后，升级成功会延迟重启两个
Go 服务；没有 `LIKEADMIN_UPGRADE_RESTART_COMMAND` 时拒绝在线应用 Go 后端包。

定时任务：

```bash
go run ./cmd/crontab          # 循环执行 la_dev_crontab
go run ./cmd/think            # 列出已迁命令
go run ./cmd/think help clear
go run ./cmd/think crontab    # 只跑一轮
go run ./cmd/think query_refund
go run ./cmd/think run --port 8000
```

`make:*`、`build`、`vendor:publish`、`service:discover` 的 PHP 脚手架已永久关闭。Go 运行时和生产部署不需要 PHP 产物。

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
