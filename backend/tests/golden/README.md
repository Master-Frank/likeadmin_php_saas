# 黄金对拍与模块验收

阶段完成定义（与 plan 一致）：**同一请求 PHP vs Go JSON 对拍通过 + 对应前端主路径走通 + Nginx 已切该前缀 + 未迁移模块仍走 PHP**。

「路由已注册 / `go test` 绿」不等于验收完成。

## 模块验收清单

| 模块 | 必测接口 | 状态 |
|---|---|---|
| 环境 | MySQL+Redis 导入 `like.sql`，Go/PHP 同库同 Redis | 未过 |
| 平台登录 | `login/account` `login/logout` `config/getConfig` `config/dict` `auth.admin/mySelf` | 未过 |
| 平台 RBAC | `auth.admin/*` `auth.role/*` `auth.menu/*` `dept.dept/*` `dept.jobs/*` | 未过 |
| 平台设置 | `workbench/index` 上传/文件 字典 网站 存储 缓存 日志 | 未过 |
| 租户生命周期 | `tenant.tenant/add` 共享表与分表、子域名登录、交叉隔离 | 未过 |
| 租户内核 | `tenantapi/login` `auth.*` 配置 文件 组织 | 未过 |
| 租户业务 | 用户 文章 装修 渠道 充值退款 通知 | 未过 |
| 用户端 | `/api` 注册登录 用户中心 文章 充值 `pay/prepay` 回调 path 短信 | 未过 |
| 定时/安装 | crontab 执行、`/install` 写 lock | 未过 |
| 全量切流 | SPA + Nginx 全切 Go，下线 php-fpm | 未做 |

允许差异：新签发 `token`、键顺序、部分统计瞬时值。不允许：`code`/`show`/`msg` 语义、列表字段、空 `data` 形态（`[]` vs `{}`）、时间格式、权限拒绝码。

## 跑对拍

```bash
# 先起 MySQL/Redis，导入 server/public/install/db/like.sql
# PHP 与 Go 都指向同一库、Redis prefix la:
export PHP=http://127.0.0.1:8000
export GO=http://127.0.0.1:8080
./backend/tests/golden/pair.sh
```
