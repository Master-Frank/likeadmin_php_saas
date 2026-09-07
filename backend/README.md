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

定时任务：

```bash
go run ./cmd/crontab
# 或
go run ./cmd/think query_refund
```

离线升级包（已下载的 zip，无需 mddai.cn）：

```bash
# ApplyLocal / file:// 与远程 link 走同一套解压+SQL+文件管道
```

## 契约

- URL：`/{platformapi|tenantapi|api}/{controller}/{action}`，嵌套控制器用点号，如 `/platformapi/auth.admin/lists`
- 响应：`{code, show, msg, data}`
- 鉴权 Header：`token`
- 密码：`md5(salt + md5(password + salt))`，salt 为 `project.unique_identification`

验收清单见 [`tests/golden/README.md`](tests/golden/README.md)。
