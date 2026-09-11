# likeadmin-SaaS Go 后端性能现状与待办

> Review 日期：2026-09-11
> 适用范围：当前 `backend/` Go 后端。PHP `server/` 仅作为静态资源、SQL 和兼容对照源。
> 本文只记录已经存在的能力和仍需实施的事项。测试通过不等于完成容量验证；仓库目前没有可复现的生产规模压测结果。

## 1. 结论

- PHP → Go HTTP 迁移已完成，307/307 个公开动作由 Go 提供；后续性能工作不再讨论重写后端。
- P0/P1 的大部分保护、首批索引、热路径缓存、查询降本和部署配置已经落地。
- 本轮已关闭先前的上线阻断点中的权限 fail-open、tenant 导出轮询、操作日志隔离/脱敏/缓冲、可信代理、Redis 安全状态 fail-closed，以及启动期串行建索引。
- 真正的异步导出 worker（游标读取、共享/对象存储、请求内不再物化 10000 行）和可复现容量基线仍未完成。
- `go test ./...`、`go vet ./...` 是功能与回归证据，不是吞吐、延迟或容量证明。
- 精确 `count`、`page_size_max=25000` 和 PHP 兼容响应契约仍保留。

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
- 可配置一个只读副本；探活在后台刷新健康状态，不再在请求 goroutine 里持锁等待 Ping。不可用时读请求回落主库。
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
- 平台租户列表对共享 `user` 表使用一次 `GROUP BY` 计数；`tactics=1` 分表租户仍逐租户计数。
- 平台端和租户端 `PayWayGet` 先收集 `pay_config_id`，一次 `IN (?)` 查询配置。
- 文章浏览量使用数据库原子自增。
- GET/HEAD 和 `download/*` 不安装 response capture writer；非 GET 使用 64 KiB 上限的边写边截断 buffer。
- 操作日志表有 `tenant_id`；租户日志按 tenant ID 过滤。列尚未升级时租户查询失败关闭（空列表），不会回退到 admin ID + URL 近似隔离。
- 参数脱敏递归处理 map/list，覆盖 password/secret/private_key/mch_key/access_key_secret 等 credential-shaped key。
- `LIKEADMIN_OPLOG_ASYNC=1` 时操作日志进入 256 长度的进程内有界队列；队列满时丢弃。
- 普通列表仍返回精确 `count`；没有改成 `has_more` 或估算值。
- `page_type=0` 仍可读取 `page_size_max`，默认上限仍为 25000。

### 2.5 导出当前实现

- 导出预览和提交都受以下硬限制：
  - `export_max_pages` 默认 20；
  - `export_max_rows` 默认 10000；
  - 超限直接报错，不再静默截断。
- task ID 和 file key 使用加密随机数；任务状态保存 Redis 30 分钟。
- 下载 URL 按 app 返回 `/platformapi` 或 `/tenantapi`，带 HMAC 签名和过期时间；tenant Vue 轮询 `/tenantapi/download/export`，非成功 `code` 立即失败。
- 任务轮询绑定创建管理员和租户；文件下载校验签名，或校验已登录 owner。
- 文件写入 `public_dir` 同级的 `runtime/export`，不再暴露在匿名 `/uploads` 静态目录。
- 打开并确认文件可读后才删除一次性 file key；下载成功后删除磁盘文件；janitor 按任务 TTL 清理过期文件。
- 单进程最多同时执行 2 个 XLSX 写盘任务，并有 panic 恢复。
- 当前“异步”边界仍只覆盖 XLSX 组装和写盘：列表 SQL、关联组装以及最多 10000 行结果仍在原 HTTP 请求内完成。
- 文件仍写本机目录；Redis 中保存的是该实例的绝对路径。没有共享盘或对象存储时，另一实例无法下载。

### 2.6 多实例、静态资源与 CDN

- 安装向导可选择单实例/多实例、单库/主从，并写入 Go 配置。
- 多实例强制要求 Redis；安装完成后会重新连接 DB 和 Redis。
- session、权限、限流、导出任务等安全 key 在 Redis 已配置或被要求时不回落进程内 `sync.Map`；公开 boot/config 仍允许短 TTL 本地缓存。
- nginx upstream 可增加多个 Go 实例；crontab 已有 MySQL advisory lock。
- 本地上传仍要求运维提供共享盘，或在后台配置 OSS。
- `app.cdn_domain` 可配置文件 CDN 域名。
- nginx 已启用 gzip；`resource/uploads` 可设置长期缓存；带扩展名的 SPA asset 跳过租户解析。
- `/pages`、`/packages` 店铺链接和 PC 图片双斜杠兼容已修复。

### 2.7 当前可观测性与验证

- GORM callback 可以把使用 request context 的 SQL 数量和耗时挂到请求。
- `LIKEADMIN_METRICS=1` 时，`127.0.0.1:9090/metrics` 当前只输出：
  - HTTP 请求总数；
  - SQL 查询总数；
  - export pending/ready/failed 累计数；
  - instance 标签。
- `backend/tests/performance/` 目前只有 query stats 挂载和导出上限配置测试，不是 benchmark 或负载测试。
- 尚无 HTTP 延迟直方图、DB pool wait、Redis hit/miss/fallback、导出在飞数量、Go runtime 指标或固定数据集压测结果。
- 因此当前不能给出可信的单机 QPS、p95/p99、容量上限或“提升倍数”。

## 3. 待办

### P0-2 完成真正的异步导出和多实例文件闭环

已完成：tenant 轮询 URL、签名下载、打开后再消费 file key、下载后删文件、按年龄 janitor。

仍待实施：

1. HTTP 只保存导出条件、稳定排序字段、tenant/admin 身份和导出字段，立即返回 task ID。
2. 独立 worker 从 Redis/数据库领取任务，重新建立租户上下文。
3. 使用稳定主键游标分批读取；禁止把整个导出结果放进 HTTP 请求或单个 Go slice。
4. CSV/XLSX 流式写出，限制每租户并发、总行数、文件大小、执行时长和保留时间。
5. 多实例使用 OSS/S3 或明确挂载的共享目录；任务元数据记录对象 key，不保存实例绝对路径。
6. worker crash 后任务应超时失败或可重试；进程退出前停止领任务并处理租约。

验收：

- 创建任务接口的延迟不随导出行数线性增长；
- worker 处理大导出时普通 API 的 p99 和内存保持在预算内；
- 任意实例都能轮询和下载同一任务；
- 其他管理员、其他租户和匿名请求无法读取任务或文件。
- 下载和未下载文件都在保留期后清理。

### P0-5 建立可复现性能基线

现状只有功能测试和累计计数器，无法证明容量。

实施：

1. 在 `backend/tests/performance/` 增加可执行的 k6/vegeta 场景：
   - `GET /api/index/config` 冷/热缓存；
   - 文章列表首页、分类和 keyword；
   - 已登录平台/租户管理员列表；
   - 用户中心；
   - 登录、短信 stub、支付沙箱；
   - 大列表与导出单独压测。
2. 固定小、中、大三档数据集，记录热点租户比例和核心表基数。
3. 指标至少补充：
   - HTTP RPS、错误率、在飞请求、p50/p95/p99；
   - SQL/请求、SQL 时延、`sql.DB.Stats()`；
   - Redis hit/miss/error/fallback、命令时延；
   - Go heap、alloc、GC pause、goroutine；
   - export queue/in-flight/duration/file size。
4. 确保所有请求 DB session（包括 `bootstrap.Read()` 路径）绑定 request context，否则 query counter 会漏记。

验收：

- 相同 commit、配置、数据集和机器可重复得到结果；
- 冷缓存、热缓存、Redis 故障和 replica 故障分别有基线；
- 容量报告只引用实测数据，不再使用 PHP 经验倍数。

### P0-7 生产索引迁移的可观测升级步骤

启动路径已不再默认同步建索引。仍待：

1. 将 DDL 放入显式、幂等、可观测的升级步骤；按 MySQL 版本配置 online DDL 策略。
2. 发布前展示待执行表、索引、预计锁影响和执行结果。
3. 应用启动只校验必要 schema version，不修改大表。
4. 为失败、超时和部分完成提供可重试状态，不以普通日志代替迁移结果。

### P1-1 收紧普通列表上限与深分页

现状：

- 导出已限制为 20 页/10000 行；
- 普通 `page_type=0` 仍直接使用 `page_size_max=25000`；
- 多数列表仍执行精确 `COUNT(*)`，深分页仍用 OFFSET。

实施：

1. 区分普通列表上限和兼容/内部全量读取上限，普通 API 使用更小硬限制。
2. 逐个确认四套前端是否仍发送 `page_type=0`，不能直接全局改语义。
3. 大表列表增加稳定 keyset/cursor；深页避免大 OFFSET。
4. 精确 count 暂不改变；先用慢查询和压测确定高成本列表，再讨论首屏 count、异步统计或新版本契约。

验收：

- 任意普通公网/后台列表都不能单请求物化 25000 行；
- 调整不破坏当前精确 `count` 契约和 golden pair 门禁。

### P1-2 操作日志可靠性与退出处理

现状：

- 可选异步队列只有 256 项；
- 队列满静默丢弃；
- 单条 INSERT，没有批量写；
- 进程退出不 drain；
- 没有 dropped/queued/duration 指标。

实施：

1. 明确安全审计事件与低价值 GET 日志的不同可靠性等级。
2. 写操作/登录/权限变更等审计事件不能静默丢弃。
3. 增加批量写、队列指标、失败重试/持久化策略和优雅停机 drain。

验收：

- 队列拥塞和 DB 故障有指标与告警；
- 安全审计日志满足明确的保留与可靠性要求；
- 日志高峰不明显抬高普通 API p99。

### P1-3 查询、缓存与只读副本细化

已完成：PayWay `IN (?)`、boot 只 bump 版本、host 规范化并纳入 scheme、replica 探活不持锁、向导文案。

仍待：

1. 平台租户列表为 `tactics=1` 分表租户逐个 COUNT；应提供批量汇总来源、缓存统计或明确限制分表租户列表统计成本。
2. replica 不能只检查 TCP Ping：启动失败后应重试绑定；检查 schema version、复制状态和可接受 lag；可安全回退的读在 query error 时切回主库；暴露 replica health/lag 指标。
3. 用生产数据 `EXPLAIN ANALYZE` 验证当前首批索引。

### P1-4 静态、上传与 CDN 完成态

1. 多实例上线前将本地上传迁到 OSS，或验证所有实例共享同一挂载及权限。
2. 带内容 hash 的 SPA 文件设置 `immutable`；HTML 保持短缓存或 no-cache。
3. 上传使用独立 location、体积和超时预算，不扩大普通 API 预算。
4. CDN 只缓存公开、无用户态内容；缓存 key 必须包含 tenant、host、终端和相关 query。

### P1-5 运行期正确性与故障边界

已完成：原子限流、`/readyz` `PingContext`、DSN timeout、boot scheme、分类主动失效、HasColumn TTL、413。

仍待：

1. replica 绑定重试、复制 lag 和 query error 回退主库。
2. 普通 JSON 接口使用远小于上传的 route-specific body limit。

### P2 后续容量演进

- 工作台统计在数据证明需要时迁移到分钟级汇总或事件聚合。
- 对 `%keyword%` 查询先验证产品能否改为前缀检索，再决定全文索引/搜索服务。
- cron 全租户扫描和 `information_schema` 探测按指标决定是否分批和缓存。
- 只有在缓存、索引、查询和读副本优化后主库仍长期达到瓶颈，才评估独立库、哈希分片、TiDB/Vitess。
- `tactics=1` 的租户分表是兼容/隔离能力，不作为默认性能方案。

## 4. 实施顺序

1. ~~修复权限 DB 错误 fail-open。~~
2. 修复 tenant task 轮询（已完成）后，把导出改为独立 worker + 游标读取 + 共享/对象存储。
3. ~~给操作日志增加 tenant ID、递归脱敏，并移除 GET/下载/大响应全量缓冲。~~
4. ~~将 Go 监听限制在可信代理边界内。~~
5. ~~多实例安全状态在 Redis 运行期故障时失败关闭。~~
6. ~~把生产索引 DDL 移出应用启动。~~ 补齐可观测迁移状态。
7. 建立固定数据集、负载脚本和完整指标。
8. 用基线收紧普通列表并治理高成本 COUNT/深分页。
9. 完善日志可靠性、依赖超时和 replica 健康。
10. 根据指标处理缓存扫描、静态/CDN 和更深层数据库演进。

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
