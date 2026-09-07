# 黄金对拍与模块验收

阶段完成定义（与 plan 一致）：**同一请求 PHP vs Go JSON 对拍通过 + 对应前端主路径走通 + Nginx 已切该前缀 + 未迁移模块仍走 PHP**。

「路由已注册 / `go test` 绿」不等于验收完成。

## 模块验收清单

| 模块 | 必测接口 | 状态 |
|---|---|---|
| 环境 | MySQL+Redis 导入 `like.sql`，Go/PHP 同库 | 本机已起；PHP 用 file cache，Go 用 Redis |
| 平台登录 | `login/account` `config/getConfig` `auth.admin/mySelf` | 对拍已过（字段级） |
| 平台 RBAC | admin/role/menu/dept/jobs lists | 对拍已过 |
| 平台设置 | workbench / storage / dict / notice lists+detail / pay getConfig | 对拍已过 |
| 租户生命周期 | 共享表租户 `pair1` 已建；分表 `tactics=1` 代码已接线 | pair2 登录/文章/跨租户已对拍 |
| 租户内核 | login / getConfig / workbench / mySelf / menu / dept | 对拍已过 |
| 租户业务 | 文章/用户 lists、装修 tabbar/page；文章/分类写路径、用户 edit/adjustMoney、装修保存、网站设置、租户 notes、文件分类 CRUD、通知 detail/set、余额支付 remark 回写、公众号回复 CRUD | 对拍已接写路径 |
| 平台字典 | dict_type 校验 + add/edit/delete | 对拍已过 |
| 用户端 | `/api` config/decorate/article/search；注册+登录+center/info | 对拍已过（含注册重复、登录后 lists） |
| 定时/安装 | crontab 执行、`/install` 导入 like.sql 并写 lock/.env | 安装导入已实现；本库 lock 已存在未重装 |
| 全量切流 | `cmd/strangler` 把 API 切 Go，其余回 PHP；nginx 配置听 :8091 | strangler :8090 对拍 failed=0；nginx :8091 对拍 failed=0 |

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
