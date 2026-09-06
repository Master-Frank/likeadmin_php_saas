# likeadmin-SaaS Go 后端

渐进式替换 `server/` 下的 ThinkPHP 后端。接口前缀、JSON 信封、`token` Header、密码算法与 PHP 保持一致。

## 运行

```bash
cd backend
go mod tidy
export LIKEADMIN_CONFIG=$(pwd)/configs/config.yaml
# 按实际环境修改 configs/config.yaml 中的 MySQL / Redis
go run ./cmd/api
```

默认监听 `:8080`。可用 `LIKEADMIN_LISTEN=:8080` 覆盖。

Strangler 切流（API 走 Go，页面/未覆盖路径走 PHP）：

```bash
LIKEADMIN_STRANGLER=127.0.0.1:8090 LIKEADMIN_GO=http://127.0.0.1:8080 LIKEADMIN_PHP=http://127.0.0.1:8000 go run ./cmd/strangler
```

定时任务（循环执行 `la_dev_crontab`，`LIKEADMIN_CRON_ONCE=1` 只跑一轮）：

```bash
go run ./cmd/crontab
```

## 契约

- URL：`/{platformapi|tenantapi|api}/{controller}/{action}`，嵌套控制器用点号，如 `/platformapi/auth.admin/lists`
- 响应：`{code, show, msg, data}`
- 鉴权 Header：`token`
- 密码：`md5(salt + md5(password + salt))`，salt 为 `project.unique_identification`

平台 / 租户 / 用户四端前端 `src/api` 路径已注册。微信/支付宝预下单、V3 回调解密、阿里云/腾讯云短信网关已有实现；**尚未**用真实库做黄金 JSON 对拍，也未 Nginx 切流。验收清单见 [`tests/golden/README.md`](tests/golden/README.md)。
