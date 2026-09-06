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

定时任务：

```bash
go run ./cmd/crontab
```

## 契约

- URL：`/{platformapi|tenantapi|api}/{controller}/{action}`，嵌套控制器用点号，如 `/platformapi/auth.admin/lists`
- 响应：`{code, show, msg, data}`
- 鉴权 Header：`token`
- 密码：`md5(salt + md5(password + salt))`，salt 为 `project.unique_identification`
