# likeadmin-SaaS Go 后端性能现状与待办

> Review 日期：2026-09-11
> 适用范围：当前 `backend/` Go 后端。PHP `server/` 仅作为静态资源、SQL 和兼容对照源。
> 本文只记录已经存在的能力和仍需实施的事项。测试通过不等于完成容量验证；仓库目前没有可复现的生产规模压测结果。

## 1. 结论

- PHP → Go HTTP 迁移已完成，307/307 个公开动作由 Go 提供；后续性能工作不再讨论重写后端。
- P0/P1 的大部分保护、首批索引、热路径缓存、查询降本和部署配置已经落地。
- 本轮已关闭先前的上线阻断点中的权限 fail-open、tenant 导出轮询、操作日志隔离/脱敏/缓冲、可信代理、Redis 安全状态 fail-closed，以及启动期串行建索引。
- 导出在 `LIKEADMIN_EXPORT_ASYNC=1` 或多实例时改为 HTTP 只入队、worker 重放列表；worker 内仍最多物化 `export_max_rows`（默认 10000）行，没有改成逐列表游标 SQL。
- `go test ./...`、`go vet ./...` 是功能与回归证据，不是吞吐、延迟或容量证明。k6 脚本已提供，仓库里仍然没有实测 QPS。
- 精确 `count` 和 PHP 兼容响应契约仍保留。普通 `page_type=1` 列表硬限制 500 行；后台 `page_type=0` 仍可用 `page_size_max`（默认 25000）。

## 2. 当前状态

### 2.1 HTTP、依赖与入口保护

- `backend/internal/httpserver/server.go`
  - 显式 `http.Server`；
  - `ReadHeaderTimeout=10s`、`ReadTimeout=30s`、普通 `WriteTimeout=30s`、导出 `120s`、`IdleTimeout=60s`；
  - 1 MiB header 上限、50 MiB 应用层请求体上限；
  - 超过上限时 `httpx.Body` 识别 `http.MaxBytesError` 并返回 HTTP 413；
  - SIGINT/SIGTERM 优雅停机；
  - pprof 仅在 `LIKEADMIN_PPROF=1` 时监听 `127.0.0.1:6060`。
- `backend/internal/router/router.go`
  - `/healthz` 表示进程存活；
  - `/readyz` 使用短 deadline 的 `PingContext` 检查主库；配置要求 Redis 时也执行 Redis Ping。
- `backend/internal/ratelimit/limit.go`
  - 登录、注册、短信、上传、支付预下单、安装和代码生成已有按 IP 的 Redis 计数限流；
  - `INCR`+`EXPIRE` 在 Redis 上通过 Lua 原子执行；
  - `RequireRedisConfigured()` 为真且 Redis 不可用时限流失败关闭。
- 生产 systemd 默认 `LIKEADMIN_LISTEN=127.0.0.1:8080`。
- `X-Real-IP` / `X-Forwarded-Proto` / `X-Forwarded-Host` 仅在 `RemoteAddr` 属于本机或 `LIKEADMIN_TRUSTED_PROXIES` 时读取；忽略客户端 `X-Forwarded-For`。
- Redis dial/read/write timeout 默认 200ms；多实例或显式要求 Redis 时，启动检查会失败关闭。
- MySQL DSN 带 `timeout=5s&readTimeout=10s&writeTimeout=10s`。

### 2.2 数据库连接、索引与只读副本

- MySQL 连接池参数已配置化：
  - `max_open_conns`，默认 50；
  - `max_idle_conns`，默认 10；
  - `conn_max_lifetime`，默认 300 秒；
  - `conn_max_idle_time`，默认 60 秒。
- `EnsurePerfIndexes` 已为以下查询形状创建幂等候选索引，并同步到内嵌 SQL / 安装 SQL：
  - tenant：`(sn,delete_time)`、`(domain_alias,delete_time)`；
  - config：`(type,name)`；
  - tenant_config：`(tenant_id,type,name)`；
  - article：`(tenant_id,is_show,delete_time)`；
  - user：`(tenant_id,delete_time)`；
  - operation_log：`(create_time)`、`(tenant_id,create_time,id)`。
- HTTP 启动默认不再串行 `CREATE INDEX`。安装成功后会执行一次；存量库使用 `bin/think ensure-indexes` 或 `LIKEADMIN_ENSURE_INDEXES=1`。
- `LIKEADMIN_REQUIRE_DDL=0` 时跳过建索引、加列和 DDL 权限探测。
- 可配置一个只读副本；探活在后台刷新健康状态，不再在请求 goroutine 里持锁等待 Ping。不可用或复制 lag 超过 `LIKEADMIN_REPLICA_MAX_LAG`（默认 30s）时读请求回落主库。绑定失败会在后台重试。
- 操作日志列表、部分工作台统计和 tenant 读会使用只读入口；支付、鉴权、配置和写后读仍走主库。
- 安装向导已更正：主从只把日志/统计等可延迟读打到从库，导出仍走原列表查询路径。

### 2.3 热路径缓存

- 租户元数据：
  - host/sn/id 使用 Redis + singleflight；
  - request meta 避免同一请求重复解析；
  - 写路径会失效旧域名、新域名、SN 和 ID；
  - 仅 `gorm.ErrRecordNotFound` 写 10 秒 negative cache，数据库错误不缓存为“不存在”。
- 配置：
  - request-local + Redis；
  - `GetMany` 合并同类型配置查询；
  - SQL 错误不写 miss cache；
  - storage、SMS、secret/private/cert/password/access key、公众号 token/AES 等敏感值不写普通 Redis key；
  - 保存配置后删除对应 key，并 bump boot version；不再 `SCAN boot:{tenant}:*`。
- C 端启动包：
  - `api/index/config` 使用 tenant + version + scheme + host 的 boot key；
  - 公开配置和装修响应有短 `Cache-Control` 与弱 ETag。
- 权限：
  - 全菜单和管理员菜单均先读缓存；
  - 菜单、角色、角色菜单和生成器写菜单后 bump 版本；
  - 动态生成 CRUD 必须具有显式权限；
  - 缓存中的 `all` 缺少某 URI 时，实时菜单查询成功且不存在才按 PHP 未登记路由放行；查询失败则拒绝，错误结果不进入 15 秒本地缓存。
- 装修、tabbar、分类、热搜、工作台总数已有短 TTL 缓存或增量更新；文章分类状态变更会主动 `invalidatePublic("cate")`。
- `schemacache.HasColumn` 按库身份缓存，TTL 5 分钟；查询错误不写入缓存。

### 2.4 查询与请求关键路径降本

- `api/recharge/lists` 已分页，不再无界读取。
- 平台租户列表对共享 `user` 表使用一次 `GROUP BY` 计数；`tactics=1` 分表租户仍逐租户计数，结果缓存 30 秒。
- 平台端和租户端 `PayWayGet` 先收集 `pay_config_id`，一次 `IN (?)` 查询配置。
- 文章浏览量使用数据库原子自增。
- GET/HEAD 和 `download/*` 不安装 response capture writer；非 GET 使用 64 KiB 上限的边写边截断 buffer。
- 操作日志表有 `tenant_id`；租户日志按 tenant ID 过滤。列尚未升级时租户查询失败关闭（空列表），不会回退到 admin ID + URL 近似隔离。
- 参数脱敏递归处理 map/list，覆盖 password/secret/private_key/mch_key/access_key_secret 等 credential-shaped key。
- `LIKEADMIN_OPLOG_ASYNC=1` 时操作日志进入 256 长度的进程内有界队列；GET 队列满可丢弃，POST/登录等审计事件改为同步写入。进程退出前 `DrainOplog`。
- 普通列表仍返回精确 `count`；没有改成 `has_more` 或估算值。
- 普通 `page_type=1` 列表 `page_size` 硬限制 500；C 端 `app=api` 的 `page_type=0` 同样限制 500。平台/租户后台 `page_type=0` 仍使用 `page_size_max`（菜单/字典/素材）。`ValidateQuery` 的 25000 报错文案未改。

### 2.5 导出当前实现

- 导出预览和提交都受以下硬限制：
  - `export_max_pages` 默认 20；
  - `export_max_rows` 默认 10000；
  - 超限直接报错，不再静默截断。
- task ID 和 file key 使用加密随机数；任务状态保存 Redis 30 分钟。
- 下载 URL 按 app 返回 `/platformapi` 或 `/tenantapi`，带 HMAC 签名和过期时间；tenant Vue 轮询 `/tenantapi/download/export`，非成功 `code` 立即失败。
- 任务轮询绑定创建管理员和租户；文件下载校验签名，或校验已登录 owner。
- 文件写入 `public_dir` 同级的 `runtime/export`，或 `LIKEADMIN_EXPORT_DIR`；元数据保存相对文件名，不再保存实例绝对路径。
- 打开并确认文件可读后才删除一次性 file key；下载成功后删除磁盘文件；janitor 按任务 TTL 清理过期文件。
- `LIKEADMIN_EXPORT_ASYNC=1` 或多实例时，HTTP 只保存导出条件并返回 `task_id`；后台 worker 带租户/管理员上下文重放原列表 handler。
- worker 使用 Redis 队列 `export_jobs` + `SETNX` 租约；无 Redis 的单实例走进程内队列。租约丢失的 pending 任务会再入队，最多 3 次，超时失败。每租户互斥，单进程 2 个 worker，单任务 2 分钟，文件 50MiB。
- XLSX sheet 流式写入 zip，不再先拼整张表字符串。
- 关闭异步时仍走原路径：列表查询在 HTTP goroutine 内完成。
- worker 内仍 `Find` 最多 10000 行到内存；没有为每个列表改成主键游标分批 SQL。导出文件默认不上传到公开 OSS（避免进入 CDN `/uploads`）；多实例需挂载同一 `LIKEADMIN_EXPORT_DIR`。

### 2.6 多实例、静态资源与 CDN

- 安装向导可选择单实例/多实例、单库/主从，并写入 Go 配置。
- 多实例强制要求 Redis；安装完成后会重新连接 DB 和 Redis。
- session、权限、限流、导出任务等安全 key 在 Redis 已配置或被要求时不回落进程内 `sync.Map`；公开 boot/config 仍允许短 TTL 本地缓存。
- nginx upstream 可增加多个 Go 实例；crontab 已有 MySQL advisory lock。
- 本地上传仍要求运维提供共享盘，或在后台配置 OSS。
- `app.cdn_domain` 可配置文件 CDN 域名。
- nginx 已启用 gzip；`/uploads` 与 `/resource` 长期缓存；`/(platform|admin|mobile|pc)/assets/` 使用 `immutable`；SPA HTML `no-cache`。Go `serveSPA` 同步设置这些头。
- JSON API `client_max_body_size 1m`；`/upload/` 与本地上传目录保持 50m / 120s。Go 层同样：JSON 1MiB，上传路径 50MiB。
- `/pages`、`/packages` 店铺链接和 PC 图片双斜杠兼容已修复。

### 2.7 当前可观测性与验证

- GORM callback 可以把使用 request context 的 SQL 数量和耗时挂到请求。
- `LIKEADMIN_METRICS=1` 时，`127.0.0.1:9090/metrics` 输出：
  - HTTP 总数、在飞、延迟直方图；
  - SQL 总数、延迟直方图、`sql.DB.Stats()`；
  - export pending/ready/failed 与在飞；
  - oplog queued/dropped/written；
  - Redis 错误计数；
  - replica up/lag；
  - Go heap / goroutine / GC pause；
  - instance 标签。
- `backend/tests/performance/` 含 query stats 测试和 k6 场景脚本（boot/文章/后台列表/用户中心/写路径/导出）。脚本可重复跑，仓库不包含任何实测 QPS。
- Redis hit/miss 分项和请求级 replica query-error 自动切主仍未拆开。
- 因此当前不能给出可信的单机 QPS、p95/p99、容量上限或“提升倍数”。

## 3. 待办

### P0-2 异步导出

已完成：tenant 轮询、签名下载、打开后再消费 file key、janitor、HTTP 入队、worker 重放列表、相对路径/`LIKEADMIN_EXPORT_DIR`、租约与崩溃再入队、流式 XLSX、每租户互斥和文件大小上限。

仍待（需要按列表改 SQL，本轮不做）：

- 每个导出列表改成稳定主键游标分批读取，避免 worker 内最多 10000 行的单个 slice。
- 私有导出对象存储（与公开 `/uploads` CDN 隔离）以及跨实例不共享磁盘时的下载。
- 用压测证明创建任务延迟不随行数线性增长。

### P0-5 可复现性能基线

已完成：k6 场景脚本、数据集档位说明、HTTP/SQL 直方图、DB stats、Go runtime、export in-flight、oplog 与 replica 指标。

仍待：在固定数据集和机器上实际跑出冷/热缓存、Redis 故障、replica 故障基线。容量报告只允许引用那些实测数字。

### P0-7 生产索引迁移

已完成：启动默认只校验；`LIKEADMIN_REQUIRE_INDEXES=1` 在缺失时拒绝启动；`bin/think ensure-indexes` 打印计划/锁影响并写 `runtime/index-status.json`。

仍待：按 MySQL 版本自动选择 online DDL 算法；用生产数据 `EXPLAIN ANALYZE` 验证首批索引。

### P1-1 列表上限与深分页

已完成：普通 `page_type=1` 和 C 端 `page_type=0` 硬限制 500；后台 `page_type=0` 保持 `page_size_max` 以免菜单/字典/素材被截断。

仍待：大表 keyset/cursor；精确 `COUNT(*)` 契约变更需新版本接口。

### P1-2 操作日志

已完成：POST/登录不可静默丢弃；GET 可丢；queued/dropped/written 指标；停机 drain。

仍待：批量 INSERT、失败持久化重试。

### P1-3 查询、缓存与只读副本

已完成：PayWay `IN (?)`、boot bump、replica 探活不持锁、`tactics=1` 用户数 30s 缓存、replica lag/`SHOW REPLICA STATUS`、绑定失败后台重试、replica 指标。

仍待：replica query error 立即切主；生产数据 `EXPLAIN ANALYZE`。

### P1-4 静态、上传与 CDN

已完成：hashed SPA `immutable`、HTML `no-cache`、上传独立 location/体积/超时。

仍待：多实例本地上传迁 OSS 或验证共享盘；CDN 缓存 key 含 tenant/host/终端（运维配置）。

### P1-5 运行期正确性

已完成：原子限流、`/readyz` `PingContext`、DSN timeout、JSON 1MiB / 上传 50MiB、replica lag 与绑定重试。

仍待：读请求在 replica query error 时立即回主库。

### P2 后续容量演进

- 工作台统计在数据证明需要时迁移到分钟级汇总或事件聚合。
- 对 `%keyword%` 查询先验证产品能否改为前缀检索，再决定全文索引/搜索服务。
- cron 全租户扫描和 `information_schema` 探测按指标决定是否分批和缓存。
- 只有在缓存、索引、查询和读副本优化后主库仍长期达到瓶颈，才评估独立库、哈希分片、TiDB/Vitess。
- `tactics=1` 的租户分表是兼容/隔离能力，不作为默认性能方案。

## 4. 实施顺序

1. ~~修复权限 DB 错误 fail-open。~~
2. ~~tenant 轮询 + 独立 worker 入队；~~ 游标分批 SQL 与私有对象存储仍待。
3. ~~操作日志 tenant ID、递归脱敏、GET 不缓冲。~~
4. ~~Go 监听限制在可信代理边界内。~~
5. ~~多实例安全状态 Redis 故障失败关闭。~~
6. ~~生产索引 DDL 移出启动并补齐计划/状态文件。~~
7. ~~指标与 k6 脚本。~~ 固定数据集实测仍待。
8. ~~收紧普通列表 500。~~ COUNT/深分页仍待数据。
9. ~~日志可靠性与 replica lag。~~ query error 切主仍待。
10. 根据实测指标处理缓存扫描和更深层数据库演进。

## 5. 容量报告模板

```text
版本：
  commit、配置、实例数、CPU、内存
依赖：
  MySQL/Redis 规格、版本、网络位置
数据：
  租户数、热点租户占比、核心表行数
流量：
  接口权重、登录比例、冷/热缓存
结果：
  RPS、错误率、p50/p95/p99
  SQL/请求、MySQL CPU/连接/扫描行
  Redis 命中率/错误/fallback
  Go CPU/heap/GC/goroutine
瓶颈：
  应用、MySQL、Redis、外部服务或带宽
```

“目标 1 万 QPS”必须先拆成真实接口权重。公开配置、装修和静态资源可以由缓存/CDN 承担；登录、支付、短信、搜索和导出不能共用一个未经拆分的 QPS 目标。

迁移完成状态和生产切换约束见 [`docs/php-to-go-status.md`](docs/php-to-go-status.md)；Go 运行与部署见 [`backend/README.md`](backend/README.md)。
