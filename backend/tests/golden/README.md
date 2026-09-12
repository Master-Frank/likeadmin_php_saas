# 黄金对拍与模块验收

项目级迁移状态、PHP 对照 tag：[docs/php-to-go-status.md](../../../docs/php-to-go-status.md)。

阶段完成定义：**Go JSON 契约通过 + 对应前端主路径走通 + Nginx 已切该前缀到 Go**。

默认 `./backend/tests/golden/pair.sh` 只打 Go `:8080`（`PHP` 默认等于 `GO`）。历史 PHP 双跑需要 checkout tag `php-reference-final-20260912` 到 worktree 并显式设置 `PHP=http://127.0.0.1:8000`。无真实支付/短信凭证时只覆盖失败与校验语义。

「路由已注册 / `go test` 绿」不等于验收完成。

## 模块验收清单

前端路由硬编码白名单由 `backend/internal/router/coverage_test.go` 守门。Go 多出来的路由必须落在「Vue 两端共用 / 已注释 PHP 动作」白名单。

| 模块 | Go | 状态 |
|---|---|---|
| 环境 | `bootstrap` / `install` | 契约已过 |
| 平台登录/RBAC/组织 | `platformapi/login.go` `admin.go` `menu_role.go` `dept_jobs.go` | 契约已过 |
| 平台租户生命周期 | `platformapi/tenant.go` `tenantdb` | 契约已过（分表走 shard；删除/停用比 PHP 多清理） |
| 平台设置 | `platformapi/setting.go` `extra.go` | 契约已过 |
| 代码生成 / 升级 | `generator/`（内嵌 stub）`gencrud/` `upgrade/` | 生成写入 Vue + 菜单 + gencrud 运行时 |
| 定时/安装 | `cron/` `cmd/crontab` `install/` | 三件系统任务已 `EnsureNativeJobs` 入库 |
| 租户内核 | `tenantapi/core.go` `auth.go` `org.go` | 契约已过 |
| 租户业务 | `tenantapi/core.go` `extra.go` `channel.go` `file.go` `pay.go` | 契约已过 |
| 用户端 `/api` | `openapi/` | 契约已过 |
| 支付/短信/微信/存储 | `pay/` `sms/` `wechat/` `storage/` `filesvc/` | 已迁；未使用的 AliPay transfer / silentLogin 不迁 |
| 中间件 | `middleware/` | 已迁 |
| Think CLI | `cmd/think` `cron/think_*.go` | 已迁；不 exec PHP；PHP 脚手架永久关闭 |
| 全量切流 | `cmd/strangler` `deploy/nginx.local.conf` | 静态根为 `public/` |

允许差异：新签发 `token`、键顺序、工作台随机演示曲线。不允许：`code`/`show`/`msg` 语义、列表字段、空 `data` 形态、时间格式。

## 对拍覆盖

`pair.sh` + `pair-gap.sh` + `pair-generator-zip.sh` 默认只打 Go。无真实微信/支付宝/短信凭证时只覆盖失败与校验语义。

刻意不对拍 / 不迁：

- `LoginLogic::silentLogin`（无路由）
- AliPay `transfer` / `transferQuery`（无控制器调用）
- `api/pay/notifyApp`（Go 为微信 App 回调 URL 多注册，与 `notifyMnp`/`notifyOa` 同处理器）
- 本仓库无核销订单业务表时 `verification_orders` 为空跑
- 安装向导后端是 Go（`GET/POST /install`）

生成器 `generate_type=1` 只写 Vue/菜单/`backend/internal/generated`。

`like.sql`、`tenant.sql`、`tenantData.sql` 已内嵌在 `backend/internal/sqlassets/`。

## 跑对拍

```bash
export GO=http://127.0.0.1:8080
export TENANT_HOST=pair1.likeadmin.test
export TENANT_ACCOUNT=pair1
export MYSQL_DATABASE=likeadmin_saas   # default is localhost_likeadmin
./backend/tests/golden/pair.sh

# 生产 Nginx 切流（与 Go strangler 错开端口）
# nginx -c /workspace/backend/deploy/nginx.local.conf
export GO=http://127.0.0.1:8091
./backend/tests/golden/pair.sh
```
