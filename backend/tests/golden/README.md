# 黄金对拍与模块验收

阶段完成定义（与 plan 一致）：**同一请求 PHP vs Go JSON 对拍通过 + 对应前端主路径走通 + Nginx 已切该前缀 + 未迁移模块仍走 PHP**。

「路由已注册 / `go test` 绿」不等于验收完成。

## 模块验收清单

PHP 源文件与 Go 路由的 1:1 清单由 `backend/internal/router/php_module_test.go` 守门：公开动作 **307/307**，Logic/Lists/Validate/中间件/服务/缓存/命令均已入表。新增 PHP 文件未登记会失败；Go 多出来的路由必须落在「Vue 两端共用 / 已注释 PHP 动作」白名单，避免漏迁或重复实现。

| 模块 | PHP 源 | Go | 状态 |
|---|---|---|---|
| 环境 | `like.sql` + Redis | `bootstrap` / `install` | 对拍已过 |
| 平台登录/RBAC/组织 | `platformapi/logic/{Login,auth,dept}` | `platformapi/login.go` `admin.go` `menu_role.go` `dept_jobs.go` | 对拍已过 |
| 平台租户生命周期 | `TenantLogic` `TenantAdminLogic` `TenantCreatService` | `platformapi/tenant.go` `tenantdb` | 对拍已过（分表走 shard；删除/停用比 PHP 多清理） |
| 平台设置 | storage/dict/notice/pay/web/user/system | `platformapi/setting.go` `extra.go` | 对拍已过 |
| 代码生成 / 升级 | `GeneratorLogic` `UpgradeLogic` | `generator/`（内嵌 stub）`gencrud/` `upgrade/` | 生成写入 Vue + 菜单 + gencrud 运行时，不再写 PHP 后端文件 |
| 定时/安装 | `Crontab` `QueryRefund` `public/install` | `cron/` `cmd/crontab` `install/` | `query_refund`/`cancel_unpaid_orders`/`verification_orders` 已注册；独立 worker 含 `route:list` |
| 租户内核 | login/config/workbench/RBAC/dept | `tenantapi/core.go` `auth.go` `org.go` | 对拍已过 |
| 租户业务 | 文章/用户/装修/渠道/财务/充值/文件/通知 | `tenantapi/core.go` `extra.go` `channel.go` `file.go` `pay.go` | 对拍已过 |
| 用户端 `/api` | `api/logic/*` + lists | `openapi/` | 对拍已过 |
| 支付/短信/微信/存储 | `common/service/{pay,sms,wechat,storage}` | `pay/` `sms/` `wechat/` `storage/` `filesvc/` | 已迁；未使用的 AliPay transfer / silentLogin 不迁 |
| 中间件 | Login/Auth/Demo/CORS/租户识别/操作日志 | `middleware/` | 已迁 |
| Think CLI | `php think` + console.php | `cmd/think` `cron/think_*.go` | 已迁；不 exec PHP |
| 全量切流 | nginx / strangler | `cmd/strangler` `deploy/nginx.local.conf` | 直连/切流/Nginx 对拍 failed=0；PHP 回落默认关 |

允许差异：新签发 `token`、键顺序、工作台随机演示曲线。不允许：`code`/`show`/`msg` 语义、列表字段、空 `data` 形态、时间格式。

## 跑对拍

```bash
export PHP=http://127.0.0.1:8000
export GO=http://127.0.0.1:8080
export TENANT_HOST=pair1.likeadmin.test
./backend/tests/golden/pair.sh

# 切流代理（API 走 Go，其余回 PHP）
export GO=http://127.0.0.1:8090
./backend/tests/golden/pair.sh

# 生产 Nginx 切流（与 Go strangler 错开端口）
# nginx -c /workspace/backend/deploy/nginx.local.conf
export GO=http://127.0.0.1:8091
./backend/tests/golden/pair.sh
```
