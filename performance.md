# likeadmin-SaaS 当前 Go 后端性能优化方案

> Review 日期：2026-09-11
> 适用对象：本仓库当前 `backend/` Go 后端，而不是历史 ThinkPHP 运行时。
> 本文是基于代码和 SQL dump 的静态审查，不是压测报告。所有容量数字必须由真实数据量、真实流量模型和生产同规格环境的压测得出。

## 1. Review 结论

旧方案的方向大体正确，但基线已经变化：

- PHP → Go 已完成，307/307 个公开 HTTP 动作都由 Go 提供；“是否重写 Go、重写工期、PHP-FPM 上限”不再是待选方案。
- Go 已用静态分表表名集合，不再像 PHP `BaseModel` 那样每条 ORM 查询查询表结构，因此旧方案 C1 已完成。
- token、session 和部分权限结果已缓存；但租户解析、配置、装修等主热路径仍会频繁访问 MySQL。
- MySQL 连接池已有 `MaxOpenConns=50`、`MaxIdleConns=10`，但不可配置，也没有连接生命周期参数。
- 当前没有可复现的性能基线、应用指标、GORM 查询指标或受保护的 pprof。旧文中的 QPS、延迟和倍数都是 PHP 经验值，不能继续作为当前 Go 后端容量结论。

当前优先级不是继续换技术栈，而是：

1. 先建立可复现基线和可观测性。
2. 修复明显的无索引查询、请求级 SQL 放大和无界请求。
3. 缓存租户、配置和 C 端启动包，并做好主动失效。
4. 将同步导出、操作日志和本地临时文件移出请求关键路径。
5. 完成多实例前置条件后再横向扩容。

## 2. 旧方案逐项校正

| 旧项 | 当前状态 | Review 后处理 |
|---|---|---|
| A1 Go 兼容重写 | **已完成** | 从优化路线删除；只保留 307 路由和 golden pair 回归门禁 |
| A2 多实例 Go | 技术上可运行，但尚未完全无状态 | 保留为 P2；先处理本地导出文件、缓存失效和 Redis 可用性 |
| B1 租户元数据缓存 | **未实现** | 提升为 P0；`InstallAndTenant` 每个租户请求仍查 `la_tenant` |
| B2 配置缓存 | **未实现** | 提升为 P0；`cfgsvc.Get` 每次调用执行一条 SQL |
| B3 菜单/字典/装修缓存 | 部分实现 | 权限缓存仍每次先查全量菜单计算指纹；字典、装修未缓存 |
| B4 C 端 boot 整包缓存 | **未实现** | P0；`api/index/config` 是最明确的 SQL 放大入口 |
| B5 本机缓存 + singleflight | 有进程内 fallback，无 singleflight | 作为 Redis 后的短 TTL L1；不能代替多实例共享缓存 |
| B7 token 保持 Redis | **已实现** | 保留约束；生产 Redis 不应静默降级为长期 DB 回源 |
| C1 禁止每 SQL 查表结构 | **已完成** | `tenantdb.shardable` + GORM callback 已替代动态表结构查询 |
| C2 复合索引 | **明显不足** | P0；先按真实 SQL 和 `EXPLAIN ANALYZE` 补索引 |
| C3 弱化 COUNT | 未实现 | P1；多数列表仍 `COUNT` + 分页查询 |
| D3 导出异步 | 未实现 | P0；当前生成 XLSX 在 HTTP 请求内同步完成 |
| D4 工作台汇总 | 未实现 | P1；当前工作台仍实时 `COUNT` |
| E1 Go 连接池 | 部分完成 | 参数化并补 `ConnMaxLifetime` / `ConnMaxIdleTime` |
| E2 超时与限流 | 仅外部客户端有部分超时 | P0；HTTP server、关键入口限流和 Redis 降级策略仍缺失 |
| E3/E4 OSS/CDN | 产品能力部分具备 | 根据静态流量占比实施，不先假设收益 |

## 3. 当前代码中的明确热点

### 3.1 每个租户请求先查租户，且关键列没有索引

`backend/internal/middleware/cors.go` 的 `resolveTenant`：

1. 先按 `domain_alias + delete_time` 查询；
2. 未命中后再按 `sn + delete_time` 查询。

当前 `like.sql` 中 `la_tenant` 只有主键，没有 `domain_alias` 或 `sn` 索引。租户数增长后，这会成为所有 `/tenantapi` 和 `/api` 请求的固定扫描成本。

`tenantdb.ForTenant` / `ForTenantOn` 还会按 tenant ID 再查 `sn,tactics`。请求主路径的 `tenantdb.Use(c)` 已使用 request meta，不会重复查；非 HTTP、支付和定时任务调用 `ForTenant*` 时仍可能重复回源。

### 3.2 配置和文件域名造成请求级 SQL 放大

`backend/internal/cfgsvc/config.go` 的 `Get` 没有缓存，每次读取一行：

- 平台配置：`la_config(type,name)`；
- 租户配置：`la_tenant_config(tenant_id,type,name)`。

两张表当前都缺少对应复合索引。

`backend/internal/openapi/user.go` 的 `IndexConfig` 会直接或间接调用多次 `cfgsvc.Get*`。`filesvc.GetFileURL` 又会读取默认存储引擎和引擎配置。一次 C 端启动请求因此可能产生十几条配置 SQL，再加租户和装修查询。

这是当前最值得先优化的读路径，比直接上从库或分库更优先。

### 3.3 权限“缓存”仍每次查询数据库

`backend/internal/middleware/auth.go` 的 `cachedURIList` 会先执行 `load()` 查菜单，再计算指纹，之后才读缓存。

结果是：

- 非 root 管理员每个鉴权请求仍至少查一次完整菜单；
- Redis 只省掉部分角色菜单查询，没有消除全菜单查询；
- `DelPrefix` 使用 Redis `SCAN`，权限写入频繁时会增加额外开销。

正确方向是写路径主动失效，读路径先读缓存；TTL 只作为漏失效的兜底。

### 3.4 列表、导出和个别接口仍可能拉取大量数据

- 多数列表执行 `COUNT(*)` 后再 `SELECT ... LIMIT`。
- `project.lists.page_size_max` 当前为 **25000**；`page_type=0` 会直接采用该上限。
- `api/recharge/lists` 为兼容 PHP，当前不使用 `LIMIT`。
- `export=2` 在请求内同步组装全部记录并生成 XLSX。

这些问题不一定降低普通接口 p50，但会显著恶化内存、GC、数据库连接占用和 p99。

### 3.5 操作日志位于请求关键路径

`backend/internal/middleware/oplog.go` 对平台和租户后台请求：

- 在内存中复制响应体，最多保留约 64 KiB；
- 请求处理完成后同步插入 `la_operation_log`；
- `la_operation_log` 只有主键，按时间、管理员查询日志会逐渐变慢。

后台流量不高时影响有限，但大响应、导出或日志表增长后会放大尾延迟和主库写压力。

### 3.6 Redis 降级和多实例存在边界

`backend/internal/cache/store.go` 已有进程内 `sync.Map` fallback。它适合单实例短时兜底，但：

- Redis 故障时每个实例的 token、权限和限流状态会分裂；
- cache 调用使用 `context.Background()`，缺少请求级超时；
- `bootstrap.initRedis` 仅记录 Ping 失败，生产仍继续启动；
- 缓存击穿时没有 singleflight；
- 本地 fallback 没有容量上限或淘汰策略。

生产多实例应把 Redis 视为关键依赖，并明确“失败关闭、短时降级或只读降级”的策略。

### 3.7 HTTP server 和 Nginx 缺少性能保护参数

`cmd/api` 使用 Gin `Run` 启动默认 `http.Server`，没有：

- `ReadHeaderTimeout`、`ReadTimeout`、`WriteTimeout`、`IdleTimeout`；
- 显式的 header 大小上限；
- 优雅停机；
- 全局并发上限或关键接口限流。

生产 Nginx 配置也没有明确 proxy timeout、响应压缩、静态缓存头或限速策略。

外部微信、支付、短信和存储客户端已有 8–30 秒不等的 timeout，这是正确基础，但仍应按调用类型统一预算并传递 request context。

### 3.8 当前缺少性能证据

仓库没有 benchmark、k6/vegeta/wrk 场景、Prometheus/OpenTelemetry、pprof 接入或查询计数门禁。当前不能负责任地声称：

- Go 单机能达到某个固定 QPS；
- 某项缓存一定提升 2–4 倍；
- 4C8G 的 p99 必然落在某个区间。

这些数字必须由第 4 节基线产生。

## 4. P0：先建立基线与保护边界

### P0-1 可复现压测与可观测性

新增独立的 `backend/tests/performance/`，至少固定以下场景：

| 场景 | 目的 |
|---|---|
| 匿名 `GET /api/index/config` | 配置、装修、租户解析热读 |
| 匿名文章列表：首页、分类、keyword | COUNT、排序、索引和模糊搜索 |
| 已登录租户管理员列表接口 | token、权限、COUNT + SELECT |
| 用户详情或个人中心 | token 缓存与租户共享表 |
| 登录、支付沙箱、短信 stub | 外部依赖隔离后的尾延迟 |
| 大列表和导出 | 内存、GC、连接占用和稳定性，不与普通 QPS 混算 |

测试数据至少分三档：小租户、百万级共享业务表、热点租户。每次结果记录：

- HTTP：RPS、错误率、p50/p95/p99、在飞请求；
- Go：CPU、heap、alloc、GC pause、goroutine；
- MySQL：每请求查询数、查询 p95、扫描行数、连接池等待、慢查询；
- Redis：命中率、命令 p95、连接错误、fallback 次数；
- 业务：缓存 key 数、导出队列长度、外部调用耗时。

基线统一使用 `LIKEADMIN_DEBUG=false`，预热连接后分别记录冷缓存和热缓存结果；后台接口要说明操作日志是否开启，避免不同口径互相比较。

建议先增加：

1. HTTP 指标 middleware；
2. GORM callback/plugin 统计每请求查询数和耗时；
3. `sql.DB.Stats()`；
4. Redis hit/miss/fallback 指标；
5. 只监听管理网或独立管理端口的 pprof。

不要把 pprof、metrics 或 debug 路由直接暴露在公网 API 域名。

### P0-2 HTTP、Redis 和大请求保护

1. 用显式 `http.Server` 替代 `r.Run`，配置 header/read/write/idle timeout 和优雅停机。
2. Nginx 增加与应用一致的 `proxy_connect_timeout`、`proxy_read_timeout`、`proxy_send_timeout`；上传和导出使用单独 location/预算。
3. 登录、短信、上传、支付创建、生成器和安装接口按 IP/租户/用户限流。多实例限流状态放 Redis，不放本机 map。
4. 给 Redis 操作设置短 timeout，并记录 fallback；生产可配置 Redis 必须可用。
5. 将普通列表上限与导出上限拆开。普通 API 不应允许 25000 行响应。
6. 修复 `RechargeLists` 等无 `LIMIT` 路径；如前端依赖全量语义，应先改成分页契约再切换。

验收条件：

- 慢客户端不能长期占用连接；
- Redis 故障行为可预测且有告警；
- 任意普通列表都有硬上限；
- 大导出不会拖高普通 API 的 p99。

## 5. P0：先消除 MySQL 固定放大

### P0-3 基于查询形状补索引

以下是代码审查得到的**候选索引**，不是可直接上线的最终 DDL：

| 表 | 当前查询形状 | 首批候选 |
|---|---|---|
| `la_tenant` | `domain_alias=? AND delete_time IS NULL` | `(domain_alias, delete_time)` |
| `la_tenant` | `sn=? AND delete_time IS NULL` | `(sn, delete_time)` |
| `la_config` | `type=? AND name=?` | `(type, name)`；确认唯一性后可设 UNIQUE |
| `la_tenant_config` | `tenant_id=? AND type=? AND name=?` | `(tenant_id, type, name)`；确认唯一性后可设 UNIQUE |
| `la_article` | tenant + show + deleted + cid + `sort,id` | 按实际高频过滤组合验证两组复合索引 |
| `la_operation_log` | 时间倒序、管理员筛选 | `(create_time,id)`、必要时 `(admin_id,create_time,id)` |
| 用户/订单/账户日志 | tenant/user/status/time/order SN | 由慢查询和 `EXPLAIN ANALYZE` 决定，不批量猜索引 |

实施规则：

- 开启 slow query，按“总耗时 = 次数 × 单次耗时”排序；
- 使用接近生产基数和分布的数据跑 `EXPLAIN ANALYZE`；
- 检查写放大和冗余索引，避免给每个搜索字段单独建索引；
- `%keyword%` 不能使用普通 B-Tree 前缀，先确认产品是否可改为 `keyword%`，再决定全文检索；
- `tactics=1` 的分表也要同步索引，不能只改共享表；
- 修改内嵌 `backend/internal/sqlassets/*.sql` 的同时同步磁盘 SQL；
- dump 只影响新安装，既有数据库必须提供幂等 upgrade structure SQL。

### P0-4 缓存租户元数据

建议缓存对象：

- `tenant:host:{host}` → `{id,sn,tactics,disable,domain_alias_enable}`；
- `tenant:sn:{sn}` → 同一对象；
- `tenant:id:{id}` → 同一对象。

策略：

- Redis TTL 60–300 秒；
- 不存在的域名使用 5–15 秒 negative cache，防止随机 Host 打库；
- 创建、更新、停用、删除、修改域名时主动删除三类 key；
- 单请求继续以 `ctxutil.RequestMeta` 为唯一来源，避免同一请求再次读取；
- `ForTenant*` 使用同一缓存，而不是另建不一致的缓存。

域名和租户状态涉及隔离，不能只依赖长 TTL。写路径失效测试必须覆盖旧域名、新域名、停用和删除。

### P0-5 缓存配置并提供批量读取

第一步先缓存非敏感配置：

- 平台：`cfg:platform:{type}:{name}`；
- 租户：`cfg:tenant:{tenantID}:{type}:{name}`；
- request-local：一次请求内相同 key 只解析一次。

`cfgsvc.Set` 成功后立即删除对应 key。配置表增加索引后，再实现：

- `GetMany(type,names...)`：一个 SQL 读取同类型多个配置；
- C 端公开 boot 包：`boot:{tenantID}:{version}`；
- 文件域名/存储默认引擎在一个请求内只组装一次；
- `api/index/config`、`tenantapi/config/getConfig`、`platformapi/config/getConfig` 使用整包结果。

支付证书、私钥、短信 secret 等敏感配置不要未经威胁建模就放入普通 Redis key；可先只做 request-local 缓存或使用加密/受限缓存。

为了防击穿，可在 Redis miss 后对 `tenantID + key` 使用 singleflight。singleflight 只是回源合并，不负责跨实例一致性。

### P0-6 修复权限缓存读路径

将 `cachedURIList` 改为：

1. 先读 Redis；
2. miss 才查菜单并写缓存；
3. 菜单、角色、角色菜单写操作提交成功后精确失效；
4. 保留较短 TTL 作为兜底；
5. 记录 miss 和 rebuild 次数。

不要在每个请求中重新查全菜单计算 MD5。也尽量不要用大范围 `SCAN + DelPrefix` 做常规失效；优先版本号 key 或精确 key。

## 6. P0：移出请求关键路径

### P0-7 异步导出

当前 `export=2` 应改为：

1. HTTP 创建导出任务，返回 task ID；
2. worker 按主键游标或稳定排序分批读取，禁止一次加载 25000+ 行；
3. 流式写 CSV/XLSX；
4. 文件上传对象存储或共享文件系统；
5. 完成后记录下载地址和过期时间；
6. 每租户限制并发任务、行数、文件大小和保留时间。

多实例之前必须处理当前 `/tmp/likeadmin-export`：任务在实例 A 生成，本地文件无法保证由实例 B 下载。不要用 sticky session 掩盖长期架构问题。

### P0-8 操作日志降本

先确认审计要求，再选择：

- 只同步记录写操作和安全事件；
- GET 读取日志采样或关闭完整 response；
- 请求内只生成结构化事件，写入有界队列；
- worker 批量写独立日志库或日志系统；
- 队列满时明确丢弃低价值读日志，不能无限占用内存；
- 对密码、token、secret 保持脱敏。

审计写操作若要求强一致，可以保留同步写，但应与普通读日志分开。

## 7. P1：在 P0 数据基础上优化

### P1-1 COUNT 与工作台

- 小表保留精确 COUNT；
- 大列表支持 `has_more` / 游标分页，或只在第一页计算 count；
- 工作台按分钟写汇总表/Redis，不在每次打开页面时实时 COUNT；
- 修改 `count` 语义前先验证四套前端，不默认认为“999+”兼容。

### P1-2 C 端公开读缓存

适合整包缓存：

- `api/index/config`；
- 装修页、tabbar、文章分类、热门搜索；
- 公共文章详情（浏览数更新需与缓存解耦）。

可增加 ETag / `Cache-Control`，再由 CDN 缓存公开、无用户态响应。缓存 key 必须包含 tenant、语言/终端和影响响应的 query；绝不能缓存带用户余额、收藏状态或登录态的数据。

### P1-3 数据库与 Redis 连接配置化

MySQL 每实例起点仍可保持 20–50 open connections，但应配置化：

- `max_open_conns`；
- `max_idle_conns`；
- `conn_max_lifetime`，小于代理/NAT/MySQL 回收时间；
- `conn_max_idle_time`。

集群总连接数必须按“实例数 × 每实例池上限”计算，并给迁移、cron 和运维连接留余量。通过 `sql.DB.Stats().WaitCount/WaitDuration` 和 MySQL CPU 决定调整，而不是直接放大到 500。

Redis 同样配置 pool、dial/read/write timeout 和最大重试，并监控池等待。

### P1-4 SQL 与对象组装

- 合并同一接口内重复的配置和 storage 查询；
- 检查 join 列表的 COUNT 是否重复 join 大表；
- 深分页改 keyset/cursor；
- 文章浏览数避免每次详情同步 read-modify-write，可异步聚合；
- 通过 query-count 测试给核心接口设预算，防止后续新增 N+1。

### P1-5 Nginx、静态资源和响应体

- 对 JSON/JS/CSS/SVG 开 gzip 或 Brotli，并设置合理最小体积；
- 带 hash 的前端资源使用长期 immutable cache；
- uploads/resource 优先 OSS + CDN；
- API 默认不缓存，公开 boot/装修接口按 P1-2 单独配置；
- 上传大小和超时使用独立 location，不扩大所有 API 的预算。

## 8. P2：满足门槛后再扩展

### 8.1 多实例 Go

上线多实例前必须满足：

- Redis 为共享 token、权限、限流和缓存来源；
- 配置/租户缓存写路径可跨实例失效，或使用版本 key；
- 导出文件不依赖单机 `/tmp`；
- 本地上传改为共享存储或 OSS；
- 进程内 L1 只有短 TTL，不承载唯一状态；
- crontab 继续使用现有 MySQL advisory lock，避免多 worker 重复执行；
- 指标能按实例和全局聚合。

满足后再用负载均衡扩 Go。若扩实例后 MySQL QPS同比增长且主库先满，说明缓存和查询放大尚未解决。

### 8.2 只读副本

仅在以下证据同时出现时考虑：

- 缓存、索引和 SQL 已优化；
- 主库瓶颈主要是可延迟的读取；
- 复制延迟符合业务容忍度。

日志列表、历史统计、异步导出可读从库；支付、余额、登录后立即读取、配置保存后回读必须走主库或一致性缓存。

### 8.3 独立库、哈希分片、TiDB/Vitess

这些不是当前 P0：

- `tactics=1` 一租户一套表是兼容和隔离能力，不应作为提高 QPS 的默认方案；
- 头部租户持续占用主库资源时，再评估独立库；
- 只有优化后未缓存读写仍长期压满单库，才评估哈希分片或分布式数据库；
- 平台跨租户查询、升级、生成器和报表成本必须计入评估。

## 9. 建议实施顺序与验收门槛

### 阶段 A：测量和保护

- 建立固定数据集和压测脚本；
- 增加 HTTP/DB/Redis/Go 指标；
- 配置 server timeout、优雅停机、限流；
- 收紧普通列表上限，修复无界列表。

验收：相同环境可重复得到基线；故障和大请求不会拖垮普通接口。

### 阶段 B：消除固定 SQL 放大

- 给 tenant/config/tenant_config 补经验证的索引；
- 租户元数据缓存；
- 配置 GetMany + request cache + Redis；
- 修复权限缓存每请求回源；
- 改造 `api/index/config` 整包读取。

验收重点不是主观“更快”，而是：

- 核心接口每请求 SQL 数显著下降；
- Redis 命中率和失效正确；
- p95/p99 改善；
- 写配置、改域名、停用租户后没有脏读或越租户。

### 阶段 C：隔离重任务

- 导出任务化和共享存储；
- 操作日志异步/分级；
- 工作台汇总；
- 深分页和 COUNT 优化。

验收：导出和日志高峰不明显影响普通 API p99，worker 有并发和资源上限。

### 阶段 D：扩实例与边缘缓存

- 完成无状态门槛；
- 多实例压测；
- 公开响应 ETag/CDN；
- 根据主库读压力决定是否增加从库。

验收：增加实例时 Go 吞吐可扩展，同时 MySQL、Redis 和错误率仍在预算内。

## 10. 容量表达方式

删除旧方案中未经当前 Go 版本压测的固定区间。后续容量报告统一写成：

```text
环境：
  Go commit / 配置 / 实例数 / CPU / 内存
  MySQL、Redis规格与网络位置
数据：
  租户数、热点租户占比、核心表行数
流量：
  各接口权重、登录比例、缓存冷/热状态
结果：
  RPS、错误率、p50/p95/p99
  SQL/请求、MySQL CPU/连接/扫描行
  Redis命中率、Go CPU/heap/GC
瓶颈：
  应用 / MySQL / Redis / 外部服务 / 带宽
```

“目标 1 万 QPS”必须先拆成接口流量模型。公开配置、装修和静态资源可通过本机缓存/CDN 承担大部分流量；支付、短信、登录、模糊搜索、实时统计和同步导出不能使用同一个 1 万 QPS 目标。

## 11. 不建议做的事

- 不再讨论重写 Go 或继续优化 PHP 运行时；PHP 只作为对照源。
- 不为性能删除 `server/`；当前仍承载静态资源和上传目录。
- 不恢复每查询动态探测表结构；现有静态 shard map 更便宜。
- 不在没有基线时承诺固定 QPS 或提升倍数。
- 不先上分库、NewSQL、ES，再处理 tenant/config 无索引和配置 SQL 放大。
- 不把本机 map 当作多实例一致性缓存。
- 不把 25000 行普通响应、同步 XLSX 或无界充值记录列表当成兼容性不可改。
- 不缓存跨租户、带用户态或含支付密钥的响应。
- 不公开 pprof、metrics、安装或 crontab 管理入口。

## 12. 首批建议任务

按收益、风险和依赖排序：

1. 建性能测试目录、核心场景和 HTTP/SQL/Redis 指标。
2. 对 `la_tenant`、`la_config`、`la_tenant_config` 用生产规模数据做 `EXPLAIN ANALYZE`，提交幂等索引迁移。
3. 实现 tenant host/id 缓存及完整失效测试。
4. 实现 `cfgsvc` request cache、Redis cache、`GetMany` 和 `Set` 失效。
5. 将 `IndexConfig` 改为 boot 整包并设置 query-count 回归门禁。
6. 修复权限缓存读路径，取消每请求全菜单 fingerprint 查询。
7. 将普通列表硬上限降到合理范围，修复无 LIMIT 接口。
8. 将导出改为 worker + 对象存储/共享存储。
9. 改显式 `http.Server`，增加 timeout、优雅停机和关键入口限流。
10. 根据新的压测结果决定操作日志异步、COUNT、CDN、多实例和只读副本的顺序。

迁移完成状态和生产约束见 [`docs/php-to-go-status.md`](docs/php-to-go-status.md)；Go 运行和部署见 [`backend/README.md`](backend/README.md)。
