# likeadmin-SaaS Go 后端

渐进式替换 `server/` 下的 ThinkPHP 后端。接口前缀、JSON 信封、`token` Header、密码算法与 PHP 保持一致。

本机黄金对拍（直连 / 切流 / Nginx）已绿。未覆盖路径不再默认回落 PHP。

## 运行

```bash
cd backend
go mod tidy
export LIKEADMIN_CONFIG=$(pwd)/configs/config.yaml
go run ./cmd/api
```

默认监听 `:8080`。可用 `LIKEADMIN_LISTEN=:8080` 覆盖。

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

定时任务：

```bash
go run ./cmd/crontab          # 循环执行 la_dev_crontab
go run ./cmd/think            # 等价 php think，列出已迁命令
go run ./cmd/think help clear
go run ./cmd/think crontab    # 等价 php think crontab，只跑一轮
go run ./cmd/think query_refund
go run ./cmd/think run --port 8000
go run ./cmd/think make:controller tenantapi@Demo
```

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
