# likeadmin-SaaS Go 后端性能现状与待办

> Review 日期：2026-09-12
> 适用范围：当前 `backend/` Go 后端。PHP `server/` 仅作为静态资源、SQL 和兼容对照源。
> 本文只记录已经存在的能力和仍需实施的事项。`go test` 通过不等于容量验证。第 2.8 节的 2H2G 数字是资源硬上限推算，不是固定数据集上的实测 QPS。

## 1. 结论

- PHP → Go HTTP 迁移已完成，307/307 个公开动作由 Go 提供；后续性能工作不再讨论重写后端。
- P0/P1 的保护、首批索引、热路径缓存、查询降本、异步导出、观测脚本和部署配置已经落地。代码里能做完的项已并入第 2 节。
- 平台/租户后台导出在 `LIKEADMIN_EXPORT_ASYNC=1` 或多实例时改为 HTTP 只入队、worker 重放列表；C 端导出保留请求内用户上下文并同步完成。worker 内仍最多物化 `export_max_rows`（默认 10000）行，没有改成逐列表游标 SQL。
- 精确 `count` 和 PHP 兼容响应契约仍保留。普通 `page_type=1` 列表硬限制 500 行；后台 `page_type=0` 仍可用 `page_size_max`（默认 25000）。
- **2 核 2GB 单机（Go + MySQL + Redis + nginx 同机）** 的规划口径：日常 10～20 个后台管理员同时操作、50～100 个 C 端用户同时浏览热缓存页面可以稳住；不要按数百并发查库或秒杀来用。详见 2.8。
- **相对优化前的提升不是一个整体 QPS 倍数。** 对照基线是 cutover 完成时的 `8755b6b4`（已有 307 路由和写死的 50 连接池，但几乎没有超时、限流、缓存、索引和导出封顶）。这次做的是把“单请求能打穿进程/数据库”改成有硬上限，并让热路径少打库。正常后台分页列表仍是精确 `COUNT(*)` + 一页数据，没有测出数量级吞吐跃迁。详见 1.1。

### 1.1 优化前后对比（相对 `8755b6b4`）

没有 BEFORE/AFTER 压测，不能写“整体快了 X 倍”。能核对的是代码路径上的机械变化。

**三类路径不要混在一起说提升：**

| 类型 | 优化前 | 优化后 | 提升怎么理解 |
| --- | --- | --- | --- |
| (a) 尾部 / 滥用 | 一个请求可以拉 25000 行、无界导出、无界充值列表、无 HTTP 超时 | 普通列表 500、导出 10000 行/20 页、充值分页、超时和 body 上限 | **最坏情况从能打穿变成有顶**。行数顶大约 50×（25000→500）；导出从可按页数×page_size 放大变成硬失败 |
| (b) 热路径少打库 | 租户解析、配置几乎每次 SQL；PayWay 每个支付方式查一次配置；租户用户数按行 COUNT；GET 也整包响应再截断写日志 | Redis + singleflight、PayWay `IN (?)`、共享表一次 `GROUP BY`、GET 不再 capture | **重复读和 N+1 变成 1 次或缓存命中**。命中后延迟会掉一个数量级，但命中率和 QPS 未测 |
| (c) 正常分页后台列表 | `COUNT(*)` + 一页 | 仍然 `COUNT(*)` + 一页，另有候选索引 | **没有数量级吞吐证明**。数据变大后是否走索引要靠 `EXPLAIN`，仓库里没有生产规模结果 |

**有代码证据的倍率（不是实测 QPS）：**

- 普通 `page_type=1` 以及 C 端 `page_type=0`：`page_size` 上限 **25000 → 500（约 50× 更少行）**。后台菜单/字典/素材的 `page_type=0` 仍是 25000，这条没变。
- 平台租户列表用户数（共享 `user` 表）：页内 **N 次 COUNT → 1 次 `GROUP BY tenant_id`**。`tactics=1` 分表仍逐租户 COUNT，但加了 30 秒缓存。
- 支付方式配置：**每个 pay_way 一次 `First` → 一次 `id IN (?)`**。
- `api/recharge/lists`：无界 `Find` → 分页，且 C 端不超过 500 行。
- 导出：请求内同步、窗口可到 `page_end × page_size`（曾可到 200×25000 量级）→ 默认 **最多 10000 行 / 20 页**，超限报错；异步时 HTTP 只入队，进程内 **2 个 worker**。
- HTTP：gin 默认 `Run`（无读写超时、无限 body）→ 读 30s / 写 30s / JSON 1MiB / 上传 50MiB。
- 操作日志：管理端 GET 也全量缓冲响应再截 64KiB 并同步 `Create` → GET 不 capture；异步批量 INSERT；按 `tenant_id` 过滤。
- 查找索引：基线 SQL 没有这批 `(sn,delete_time)` / `(type,name)` / `(tenant_id,delete_time)` 等 → 安装或 `ensure-indexes` 幂等补齐。大表上这通常比应用层合并查询更决定列表延迟，但本仓库没有 `EXPLAIN ANALYZE` 数字。
- 连接池：基线已经写死 `MaxOpenConns(50)`，**不是这次才从无限改成 50**。这次改成可配置并补了连接寿命；2H2G 同机反而应降到 15 左右。

**优化前能把机器打穿的典型方式（现在被挡住或降级）：**

- 后台把 `page_size` 调到 25000 刷列表或导出。
- 充值记录一次拉全表。
- 租户很多时，租户列表每一行打一次用户 COUNT。
- 慢客户端或大包把 goroutine / 内存拖死（无超时、无 body 上限）。
- 每个公开请求都解析租户、读配置、写操作日志缓冲。

**没有变快、或未证明变快的：**

- 精确 `count` 契约没改，深分页仍然贵。
- 导出仍最多物化 10000 行，不是游标边读边写。
- 热缓存收益取决于 Redis 命中；冷启动第一次仍然打库。
- `%keyword%`、工作台全量统计、cron 扫全租户没有改成新架构。
- 仓库里没有同一数据集上的 k6 BEFORE/AFTER，因此不能写“单机 QPS 提升了多少倍”。

**一句话：** 这次优化的主收益是 **稳定性上限和查询次数**，不是测出来的整体吞吐倍数。滥用和大对象路径提升最大（几十倍行数/查询封顶）；首页/配置等热读在缓存命中后会明显轻；日常 20 条一页的后台列表，体感取决于数据和索引，不能按 50 倍去预期。

## 2. 当前状态

### 2.1 HTTP、依赖与入口保护

- `backend/internal/httpserver/server.go`
  - 显式 `http.Server`；
  - `ReadHeaderTimeout=10s`、`ReadTimeout=30s`、普通 `WriteTimeout=30s`、导出 `120s`、`IdleTimeout=60s`；
  - 1 MiB header 上限；普通 JSON 请求体 1 MiB，`/upload/` 路径 50 MiB；
  - 超过上限时 `httpx.Body` 识别 `http.MaxBytesError` 并返回 HTTP 413；
  - SIGINT/SIGTERM 优雅停机；
  - pprof 仅在 `LIKEADMIN_PPROF=1` 时监听 `127.0.0.1:6060`。
- `backend/internal/router/router.go`
  - `/healthz` 表示进程存活；
  - `/readyz` 使用短 deadline 的 `PingContext` 检查主库；配置要求 Redis 时也执行 Redis Ping。
- `backend/internal/ratelimit/limit.go`
  - 登录、注册、短信、上传、支付预下单、安装和代码生成已有按 IP 的 Redis 计数限流；
  - 默认：登录/上传 60/分钟/IP，支付 30，短信 20，安装/代码生成 10；
  - `INCR`+`EXPIRE` 在 Redis 上通过 Lua 原子执行；
  - `RequireRedisConfigured()` 为真且 Redis 不可用时限流失败关闭。
  - debug 模式默认不启用限流；需要在 debug 压测限流时显式设置 `LIKEADMIN_RATE_LIMIT=1`。生产 systemd 强制关闭 debug。
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
  - 2H2G 同机部署不要直接用满默认 50，见 2.8。
- `EnsurePerfIndexes` 已为以下查询形状创建幂等候选索引，并同步到内嵌 SQL / 安装 SQL：
  - tenant：`(sn,delete_time)`、`(domain_alias,delete_time)`；
  - config：`(type,name)`；
  - tenant_config：`(tenant_id,type,name)`；
  - article：`(tenant_id,is_show,delete_time)`；
  - user：`(tenant_id,delete_time)`；
  - operation_log：`(create_time)`、`(tenant_id,create_time,id)`。
- HTTP 启动默认不再串行 `CREATE INDEX`。安装成功后会执行一次；存量库使用 `bin/think ensure-indexes` 或 `LIKEADMIN_ENSURE_INDEXES=1`。
- `LIKEADMIN_REQUIRE_INDEXES=1` 在缺索引时拒绝启动。`ensure-indexes` 打印计划/锁影响，写 `runtime/index-status.json`，并用 MySQL advisory lock 避免并发 DDL。
- MySQL 5.7.8+/8.0 使用 `ALGORITHM=INPLACE, LOCK=NONE`（失败回退普通 `CREATE INDEX` / 加列）。`bin/think explain-indexes` 对首批查询形状跑 `EXPLAIN`；只有 `LIKEADMIN_EXPLAIN_ANALYZE=1` 才执行 `EXPLAIN ANALYZE`。
- `LIKEADMIN_REQUIRE_DDL=0` 时跳过建索引、加列和 DDL 权限探测。
- 可配置一个只读副本；后台每 5 秒刷新 Ping + replication status，读请求只读取缓存健康状态。不可用、复制 lag 未知（默认 fail-closed）或超过 `LIKEADMIN_REPLICA_MAX_LAG`（默认 30s）时回落主库；`LIKEADMIN_REPLICA_ALLOW_UNKNOWN_LAG=1` 可显式允许无 lag 数据的托管只读端点。健康检查 timeout 由 `LIKEADMIN_REPLICA_HEALTH_TIMEOUT_MS` 控制，默认 1000ms。绑定失败会在后台重试。
- 操作日志列表、平台工作台统计以及显式调用 `tenantdb.UseRead(c)` 的 tenant 查询使用只读入口；租户元数据解析、支付、鉴权、配置和写后读仍走主库。操作日志列表接受复制延迟，不提供 read-your-write。
- 请求级 replica 连接错误会立刻标记从库不健康，并在当前 Query/Row 语句上回放到主库。
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
- 平台租户列表对 `tactics=1` 分表租户的用户数缓存 30 秒。
- `schemacache.HasColumn` 按库身份缓存，TTL 5 分钟；查询错误不写入缓存。

### 2.4 查询与请求关键路径降本

- `api/recharge/lists` 已分页，不再无界读取。
- 平台租户列表对共享 `user` 表使用一次 `GROUP BY` 计数；`tactics=1` 分表租户仍逐租户计数，结果缓存 30 秒。
- 平台端和租户端 `PayWayGet` 先收集 `pay_config_id`，一次 `IN (?)` 查询配置。
- 文章浏览量使用数据库原子自增。
- GET/HEAD 和 `download/*` 不安装 response capture writer；非 GET 使用 65535 bytes 上限的边写边截断 buffer。
- 操作日志表有 `tenant_id`；租户日志按 tenant ID 过滤。列尚未升级时租户查询失败关闭（空列表），不会回退到 admin ID + URL 近似隔离。
- 参数脱敏递归处理 map/list，覆盖 password/secret/private_key/mch_key/access_key_secret 等 credential-shaped key。
- `LIKEADMIN_OPLOG_ASYNC=1` 时操作日志进入 256 长度的进程内有界队列；普通 GET 队列满可丢弃，POST/登录/`export=2` 审计事件改为同步写入。异步路径 16 条/200ms 批量 INSERT，失败重试 3 次后再逐条写。进程退出前 `DrainOplog` 最多等待 8 秒，并计入 worker 本地批次。
- 普通列表仍返回精确 `count`；没有改成 `has_more` 或估算值。
- 普通 `page_type=1` 列表 `page_size` 硬限制 500；C 端 `app=api` 的 `page_type=0` 同样限制 500。平台/租户后台 `page_type=0` 仍使用 `page_size_max`（菜单/字典/素材）。`ValidateQuery` 的 25000 报错文案未改。

### 2.5 导出当前实现

- 导出预览和提交都受以下硬限制：
  - `export_max_pages` 默认 20；
  - `export_max_rows` 默认 10000；
  - 超限直接报错，不再静默截断。
- task ID 和 file key 使用加密随机数；任务状态保存 30 分钟，多实例/要求 Redis 时只写 Redis，单实例无 Redis 时可写进程内安全缓存。
- 下载 URL 按 app 返回 `/platformapi` 或 `/tenantapi`，带 HMAC 签名和过期时间；tenant Vue 轮询 `/tenantapi/download/export`，非成功 `code` 立即失败。
- 任务轮询绑定创建管理员和租户；文件下载校验签名，或校验已登录 owner。无 owner 的 task 不允许通过公开轮询接口读取。
- 文件写入 `public_dir` 同级的 `runtime/export`，或 `LIKEADMIN_EXPORT_DIR`；元数据保存相对文件名，不再保存实例绝对路径。
- 打开并确认文件可读后才删除一次性 file key；下载成功后删除磁盘文件；janitor 按任务 TTL 清理过期文件。
- `LIKEADMIN_EXPORT_ASYNC=1` 或多实例时，平台/租户后台 HTTP 只保存导出条件并返回 `task_id`；后台 worker 带租户/管理员上下文重放原列表 handler。C 端用户上下文不写入该 job，仍在原请求内完成并直接返回签名下载 URL。
- job 只保存列表所需的管理员身份/角色上下文，不复制 token、login IP、expire time 等 session 字段。worker 执行前重新读取管理员状态和角色并复跑权限判断。
- worker 使用 Redis 队列 `export_jobs` + `SETNX` 任务租约和 tenant/platform scope 租约；无 Redis 的单实例走进程内队列。租约丢失的 pending 任务会再入队，最多 3 次，超时失败。多实例同租户互斥，单进程 2 个 worker，同步导出信号量同样为 2，任务状态超时 2 分钟，文件 50MiB。
- timeout/ready 使用 `export_finish_*` 原子终态锁，超时后的迟到 worker 不会把 failed 覆盖成 ready；底层列表若未使用 request context，超时后 SQL 仍可能继续到自身数据库超时。handler panic 转为任务失败。
- XLSX sheet 流式写入 zip，不再先拼整张表字符串。
- 关闭异步时仍走原路径：列表查询在 HTTP goroutine 内完成。
- worker 内仍最多拼出 10000 行结果；操作日志/管理员/用户等大导出改为 500 行分步 `Find`，避免一次向 MySQL 要整窗。没有为每个列表改成主键游标。导出文件默认不上传到公开 OSS；多实例需挂载同一 `LIKEADMIN_EXPORT_DIR`。

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

- GORM callback 统计全部 GORM SQL 的总数和耗时；使用 request context 的 SQL 还会挂到该请求。只读操作日志与平台工作台已使用 `RequestReadDB(c)`，tenant 只读入口由 `UseRead(c)` 绑定 context；仍直接使用全局 `bootstrap.DB` 的 handler 只有进程级 SQL 指标，没有请求级 SQL/请求统计。副本故障后同一逻辑查询会记录失败尝试和主库重试两条 SQL。
- `LIKEADMIN_METRICS=1` 时，`127.0.0.1:9090/metrics` 输出：
  - HTTP 总数、在飞、延迟直方图；
  - SQL 总数、延迟直方图、主库与 replica `sql.DB.Stats()`；
  - export pending/ready/failed 与在飞；
  - oplog queued/dropped/written/failed；
  - Redis 层错误计数、hit/miss/fallback（进程内缓存命中不计作 Redis hit）；
  - replica configured/up/lag-known/lag；
  - Go heap / goroutine / GC pause；
  - instance 标签。
- `backend/tests/performance/` 含 query stats 测试和 k6 场景脚本（boot/文章/后台列表/用户中心/写路径/导出）。脚本可重复跑，仓库不包含任何实测 QPS。
- 本仓库开发库规模很小（当前环境约 1 个租户、6 篇文章、66 条操作日志、`user` 为 0），不能代表生产数据。MariaDB 空载 RSS 已约 230MB，说明 2GB 机器上数据库内存才是主约束。

### 2.8 2 核 2GB 服务器能撑多少并发

这是规划口径，不是 k6 实测。本仓库没有在 2H2G、固定大数据集上跑过容量报告。下面的数字来自代码硬上限、同机内存争用和 Little's Law，只用来选型；上线后必须在目标机上按第 5 节模板重测。

**先分清三个“并发”：**

| 口径 | 含义 | 2H2G 单机是否贵 |
| --- | --- | --- |
| 同时在线会话 | Redis/token 里挂着但没在点页面 | 便宜，可到数百～一千 |
| 同时在飞请求 | 同一时刻正在处理的 HTTP | 贵，受 2 核和 MySQL 连接限制 |
| 可持续吞吐 | 每秒完成的请求 | 取决于接口是否打库、是否热缓存 |

**默认拓扑：Go + MySQL + Redis + nginx 全在同一台 2H2G。** 这是 likeadmin 最常见的小站部署。2GB 要同时养活操作系统、InnoDB、连接、Redis 和 Go：

- 本环境 MariaDB 在 1.2MB 数据时 RSS 已约 230MB；2GB 机器建议 `innodb_buffer_pool_size=256M～384M`，不要再给 Go 默认 50 个 MySQL 连接。
- Go 进程空载 RSS 约数十 MB，但一次 10000 行导出、后台 `page_size_max=25000` 或连接打满会把剩余内存吃光。
- 代码默认 `max_open_conns=50`。同机 2GB 应降到 **15～20**（`LIKEADMIN_DB_MAX_OPEN`），idle 5 左右。每个 MySQL 连接在服务端还有独立内存，50 连接会直接挤爆 buffer pool。
- 导出硬限制 **2 个 worker / 2 个同步信号量**。2H2G 上应开 `LIKEADMIN_EXPORT_ASYNC=1`，避免导出占满 HTTP 线程；高峰不要并行大导出。
- 2 个 CPU 还要分给 mysqld。Go 侧有效算力通常不到 2 核。

**规划区间（同机 2H2G、热缓存已预热、错误率按 <1% 来要求）：**

| 场景 | 同时在飞 | 可持续吞吐 | 依据 |
| --- | --- | --- | --- |
| 热缓存公开读（boot/config/装修/静态） | 40～80 | 80～200 req/s | 几乎不打 MySQL，2 核 + Redis 可撑 |
| 带库的 C 端列表（文章等，分页 20） | 15～30 | 30～80 req/s | 受同机 MySQL 和 15～20 连接池限制 |
| 后台精确 `COUNT(*)` 列表 | 8～20 | 15～40 req/s | 每页至少一次 count + 一次分页，深页更差 |
| 登录 / 写路径 / 短信 / 支付预下单 | 5～15 | 8～25 req/s | 主库 + 按 IP 限流（登录 60/分钟/IP） |
| 导出 | 2 | 队列消化，不要按 RPS 规划 | 进程内 worker=2，单任务最多 10000 行 |

**业务上怎么理解：**

- **可以**：10～20 个管理员同时点后台；50～100 个 C 端用户同时打开首页/资讯（缓存命中）；几百人“在线”但只有少数人在刷接口。
- **勉强**：公开接口短时冲到约 100 req/s，且大部分走 Redis/CDN。
- **不行**：几百人同时查未缓存列表、后台拉 25000 行、多个 10000 行导出、秒杀/抢购、把默认 50 连接池打满。
- 若 MySQL/Redis 不在这台 2H2G 上、只跑 Go，公开热路径可以提高一档，但后台列表仍受对端数据库规格限制，不能按“Go 很轻”直接乘倍。

**2H2G 建议配置：**

```text
LIKEADMIN_DB_MAX_OPEN=15
LIKEADMIN_DB_MAX_IDLE=5
LIKEADMIN_EXPORT_ASYNC=1
GOMEMLIMIT=384MiB
MySQL innodb_buffer_pool_size=256M～384M
Redis maxmemory 64mb～128mb，policy allkeys-lru
不要在同机再开第二套 mysqld / 大 crontab 扫描
```

“目标 1 万 QPS”和这台机器无关。公开配置、装修和静态资源应交给缓存/CDN；登录、支付、短信、搜索和导出必须单独算。

## 3. 待办

只保留还不能只靠改业务代码关闭、需要数据、压测机或运维环境的项。

### 导出与对象存储

- 每个导出列表改成稳定主键游标分批读取，并在写出时流式消费，避免最终仍拼出最多 10000 行。
- 私有导出对象存储（与公开 `/uploads` CDN 隔离）以及跨实例不共享磁盘时的下载。
- 用压测证明创建任务延迟不随行数线性增长。

### 可复现性能基线

- 在固定数据集和目标规格机器上实际跑出冷/热缓存、Redis 故障、replica 故障基线。容量报告只允许引用那些实测数字。
- 2.8 的 2H2G 规划区间必须用同规格机器的 k6 结果替换。

### 生产索引与查询形状

- 在生产规模数据上解读 `EXPLAIN`/`EXPLAIN ANALYZE` 结果并据此增补索引。
- 确认首批索引真正被用上。

### 列表契约

- 大表 keyset/cursor；精确 `COUNT(*)` 契约变更需新版本接口。

### 操作日志

- 跨进程持久化 WAL（进程崩溃时未刷盘的批次仍会丢，须持久队列才能避免）。

### 上传与 CDN

- 多实例本地上传迁 OSS 或验证共享盘；CDN 缓存 key 含 tenant/host/终端（运维配置）。

### 后续容量演进（有数据再做）

- 工作台统计在数据证明需要时迁移到分钟级汇总或事件聚合。
- 对 `%keyword%` 查询先验证产品能否改为前缀检索，再决定全文索引/搜索服务。
- cron 全租户扫描和 `information_schema` 探测按指标决定是否分批和缓存。
- 只有在缓存、索引、查询和读副本优化后主库仍长期达到瓶颈，才评估独立库、哈希分片、TiDB/Vitess。
- `tactics=1` 的租户分表是兼容/隔离能力，不作为默认性能方案。

## 4. 实施顺序

已完成（不再作为待办）：权限 fail-open、tenant 轮询与独立 worker 入队、操作日志 tenant ID/脱敏/可靠写入、可信代理、Redis 安全状态 fail-closed、索引 DDL 移出启动、online DDL / EXPLAIN 命令、指标与 k6 脚本、普通列表 500、replica lag 与查询错误切主。

仍待：

1. 全列表主键游标导出与私有对象存储。
2. 在目标规格（含 2H2G）和固定数据集上实测，替换 2.8 规划数字。
3. 生产规模 `EXPLAIN` 解读与补索引。
4. COUNT/深分页契约、oplog WAL、上传 OSS/共享盘。
5. 根据实测指标处理缓存扫描和更深层数据库演进。

## 5. 容量报告模板

```text
版本：
  commit、配置、实例数、CPU、内存
依赖：
  MySQL/Redis 规格、版本、网络位置（同机或分离）
数据：
  租户数、热点租户占比、核心表行数
流量：
  接口权重、登录比例、冷/热缓存
结果：
  同时在飞、RPS、错误率、p50/p95/p99
  SQL/请求、MySQL CPU/连接/扫描行
  Redis 命中率/错误/fallback
  Go CPU/heap/GC/goroutine
瓶颈：
  应用、MySQL、Redis、外部服务或带宽
```

k6 入口见 [`backend/tests/performance/README.md`](backend/tests/performance/README.md)。没有填过这份模板之前，不要把 2.8 的规划区间写成“已验证容量”。

迁移完成状态和生产切换约束见 [`docs/php-to-go-status.md`](docs/php-to-go-status.md)；Go 运行与部署见 [`backend/README.md`](backend/README.md)。
