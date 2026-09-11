# likeadmin-SaaS Go 后端性能现状与待办

> Review 日期：2026-09-11
> 适用范围：当前 `backend/` Go 后端。PHP `server/` 仅作为静态资源、SQL 和兼容对照源。
> 本文只记录已经存在的能力和仍需实施的事项。测试通过不等于完成容量验证；仓库目前没有可复现的生产规模压测结果。

## 1. 结论

- PHP → Go HTTP 迁移已完成，307/307 个公开动作由 Go 提供；后续性能工作不再讨论重写后端。
- P0/P1 的大部分保护、首批索引、热路径缓存、查询降本和部署配置已经落地。
- `go test ./...`、`go vet ./...` 已通过，但当前证据主要是功能与回归测试，不是吞吐、延迟或容量证明。
- 精确 `count`、`page_size_max=25000` 和 PHP 兼容响应契约仍保留。
- 本轮复审仍发现七类上线阻断点：
  1. 权限菜单实时回源查询失败时仍可能按“未登记路由”放行。
  2. 租户异步导出错误地轮询 platform API，现有 tenant token 无法通过任务 owner 校验。
  3. 导出只把 XLSX 写盘放进 goroutine；查询仍在 HTTP 请求内，多实例文件仍在本机且没有清理。
  4. 操作日志 writer 会复制全部后台响应，而且日志表没有 tenant ID，存在跨租户日志混淆与敏感参数泄漏风险。
  5. 服务默认监听 `:8080` 且无条件信任 `X-Real-IP`，直接访问可伪造 IP。
  6. 多实例要求 Redis，但运行期 Redis 错误仍回落进程内状态，权限、会话、限流和任务会发生实例分裂。
  7. 启动过程串行创建索引，大表或分表较多时可能被 metadata lock 阻塞并拖垮发布。

## 2. 当前状态

### 2.1 HTTP、依赖与入口保护

- `backend/internal/httpserver/server.go`
  - 显式 `http.Server`；
  - `ReadHeaderTimeout=10s`、`ReadTimeout=30s`、普通 `WriteTimeout=30s`、导出 `120s`、`IdleTimeout=60s`；
  - 1 MiB header 上限、50 MiB 应用层请求体上限；
  - SIGINT/SIGTERM 优雅停机；
  - pprof 仅在 `LIKEADMIN_PPROF=1` 时监听 `127.0.0.1:6060`。
- `backend/internal/router/router.go`
  - `/healthz` 表示进程存活；
  - `/readyz` 检查主库 Ping；配置要求 Redis 时也执行 Redis Ping。
- `backend/internal/ratelimit/limit.go`
  - 登录、注册、短信、上传、支付预下单、安装和代码生成已有按 IP 的 Redis 计数限流；
  - IP 读取 nginx 覆盖的 `X-Real-IP`，忽略客户端提供的 `X-Forwarded-For`；
  - 三份生产 nginx 配置均覆盖 `X-Real-IP` 和 `X-Forwarded-For`。
- Redis dial/read/write timeout 默认 200ms；多实例或显式要求 Redis 时，启动检查会失败关闭。
- 请求体读取错误目前在 `httpx` 中被忽略，超过 50 MiB 时不保证返回 HTTP 413，可能表现为普通参数校验失败。

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
  - operation_log：`(create_time)`。
- `LIKEADMIN_REQUIRE_DDL=0` 时跳过启动建索引和 DDL 权限探测。
- 默认安装会在 HTTP 监听前同步扫描表并串行 `CREATE INDEX`；错误仅写日志。
- 可配置一个只读副本；运行期每 5 秒探活一次，不可用时读请求回落主库。
- 操作日志列表、部分工作台统计和 tenant 读会使用只读入口；支付、鉴权、配置和写后读仍走主库。

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
  - 保存配置后删除对应 key，并 bump boot version。
- C 端启动包：
  - `api/index/config` 使用 tenant + version + host 的 boot key；
  - 公开配置和装修响应有短 `Cache-Control` 与弱 ETag。
- 权限：
  - 全菜单和管理员菜单均先读缓存；
  - 菜单、角色、角色菜单和生成器写菜单后 bump 版本；
  - 动态生成 CRUD 必须具有显式权限。
- 装修、tabbar、分类、热搜、工作台总数已有短 TTL 缓存或增量更新。

### 2.4 查询与请求关键路径降本

- `api/recharge/lists` 已分页，不再无界读取。
- 平台租户列表对共享 `user` 表使用一次 `GROUP BY` 计数；`tactics=1` 分表租户仍逐租户计数。
- 平台端和租户端支付方式仍按每条 pay-way 单独查询 pay-config，N+1 尚未消除。
- 文章浏览量使用数据库原子自增。
- GET 操作日志不把 response 写入数据库；非 GET 入库内容最多约 64 KiB。
- 但 `bodyWriter` 当前仍会在内存中复制完整响应，截断发生在请求结束后。
- 操作日志表没有 `tenant_id`；租户日志查询用 admin ID 与 URL 近似隔离，不能保证跨租户安全。
- 参数脱敏仅覆盖少量顶层字段，嵌套 credential 仍可能入库。
- `LIKEADMIN_OPLOG_ASYNC=1` 时操作日志进入 256 长度的进程内有界队列；队列满时丢弃。
- 普通列表仍返回精确 `count`；没有改成 `has_more` 或估算值。
- `page_type=0` 仍可读取 `page_size_max`，默认上限仍为 25000。

### 2.5 导出当前实现

- 导出预览和提交都受以下硬限制：
  - `export_max_pages` 默认 20；
  - `export_max_rows` 默认 10000；
  - 超限直接报错，不再静默截断。
- task ID 和 file key 使用加密随机数；任务状态保存 Redis 30 分钟。
- 任务轮询绑定创建管理员和租户；tenant/platform Vue 都携带 `token`，但 tenant 组件错误地固定请求 `/platformapi/download/export`，因此 tenant 异步任务当前无法正常轮询。
- 文件写入 `public_dir` 同级的 `runtime/export`，不再暴露在匿名 `/uploads` 静态目录。
- 单进程最多同时执行 2 个 XLSX 写盘任务，并有 panic 恢复。
- 当前“异步”边界仅覆盖 XLSX 组装和写盘：列表 SQL、关联组装以及最多 10000 行结果仍在原 HTTP 请求内完成。
- 文件仍写本机目录；Redis 中保存的是该实例的绝对路径。没有共享盘或对象存储时，另一实例无法下载。
- 下载前会先删除 Redis 中的一次性 file key；请求落到没有该文件的实例时，后续无法重试正确实例。
- 下载成功和 metadata TTL 到期都不会删除磁盘文件，`runtime/export` 会持续累积。
- 最终文件 URL 仍是匿名 `download/export?file=<随机 key>`；创建人校验只覆盖 task 轮询，不覆盖文件下载。

### 2.6 多实例、静态资源与 CDN

- 安装向导可选择单实例/多实例、单库/主从，并写入 Go 配置。
- 多实例强制要求 Redis；安装完成后会重新连接 DB 和 Redis。
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

### P0-1 权限回源错误必须失败关闭

现状：

- `backend/internal/middleware/auth.go` 的 `loadPlatformMenuURIs` / `loadTenantMenuURIs` 忽略 GORM 错误。
- 当缓存中的 `all` 尚未包含新菜单，同时实时菜单查询失败时，空结果会被解释为“该 URI 未登记”，最终允许访问。
- 实时菜单列表还会把查询失败产生的空结果缓存 15 秒。

实施：

1. 菜单加载函数返回 `([]string, error)`，不能忽略数据库错误。
2. `accessURI` 不在缓存 `all` 时：
   - 实时查询明确证明“不存在”才保留 PHP 的未登记路由兼容；
   - 查询失败、DB 不可用或结果不可信时返回权限不足/服务不可用，禁止放行。
3. 错误结果不进入 15 秒本地缓存。
4. 增加平台、租户两组测试：缓存落后 + DB 错误必须拒绝；缓存落后 + 新菜单 + 无角色授权必须拒绝。

验收：

- 新增菜单从写入开始就不能被未授权管理员访问；
- Redis/DB 短时故障不会把受控菜单变成公开路由；
- 307 个静态 PHP 兼容动作的权限行为不被误改。

### P0-2 完成真正的异步导出和多实例文件闭环

现状：

- tenant Vue 把 task poll 固定到 `/platformapi/download/export`；platform middleware 不识别 tenant token，owner 校验失败后前端仍继续轮询到超时。
- controller 先执行列表 COUNT/SELECT/关联组装，再调用 `export.Maybe`；
- goroutine 只处理已经驻留内存的 `rows`；
- 10000 行记录还会转换为 `[][]string`，XLSX sheet XML 再完整驻留内存；
- `runtime/export` 是本机路径，负载均衡后的 poll/download 可能落到不同实例；
- wrong-node 请求会先消费一次性 file key，再发现本机文件不存在；
- 成功下载或 metadata 过期后都不删除文件；
- task 有创建人绑定，但最终 file key 下载没有身份绑定。

实施：

1. 立即修正 tenant 轮询到 `/tenantapi/download/export`，并让前端遇到非成功 `code` 时立即报错；增加带 tenant token 的完整 middleware 测试。
2. HTTP 只保存导出条件、稳定排序字段、tenant/admin 身份和导出字段，立即返回 task ID。
3. 独立 worker 从 Redis/数据库领取任务，重新建立租户上下文。
4. 使用稳定主键游标分批读取；禁止把整个导出结果放进 HTTP 请求或单个 Go slice。
5. CSV/XLSX 流式写出，限制每租户并发、总行数、文件大小、执行时长和保留时间。
6. 多实例使用 OSS/S3 或明确挂载的共享目录；任务元数据记录对象 key，不保存实例绝对路径。
7. 验证文件存在且可打开后才能消费一次性下载 token。
8. 下载成功后删除本地文件；增加按年龄清理的 janitor，OSS 配置生命周期。
9. 下载接口要求有效登录并校验 task owner，或生成短时、一次性的签名 URL；不能只依赖匿名随机 key。
10. worker crash 后任务应超时失败或可重试；进程退出前停止领任务并处理租约。

验收：

- 创建任务接口的延迟不随导出行数线性增长；
- worker 处理大导出时普通 API 的 p99 和内存保持在预算内；
- 任意实例都能轮询和下载同一任务；
- 其他管理员、其他租户和匿名请求无法读取任务或文件。
- 下载和未下载文件都在保留期后清理。

### P0-3 修复操作日志隔离、脱敏与全量缓冲

现状：

- `backend/internal/middleware/oplog.go` 的 `bodyWriter.Write` 无上限写入 `bytes.Buffer`。
- GET 请求最终把 `result` 置空，但此前已经复制完整响应。
- `download/export` 是后台 GET 路由，也经过 `OperationLog`；下载大文件时可能把整个文件再复制一份到 Go heap。
- `OperationLog` 模型没有 `tenant_id`；租户日志列表依赖 admin ID 和 URL 模糊匹配，不同租户常见的相同 admin ID 可能互相命中。
- 参数脱敏只处理少量顶层字段，嵌套的 storage/pay 配置以及 `private_key`、`mch_key`、`access_key_secret` 等仍可能入库。

实施：

1. 日志表增加 `tenant_id`，写日志时从 request meta 填充，并增加 `(tenant_id,create_time,id)` 索引。
2. 租户日志查询直接按 tenant ID 过滤，删除 admin ID + URL 的隔离替代方案。
3. 对 map/list 递归脱敏所有 credential-shaped key，并为嵌套支付、短信、存储、公众号配置增加测试。
4. GET/HEAD 请求不安装 response capture writer；只记录状态码、耗时和必要元数据。
5. 非 GET 使用固定上限 capture writer，超过上限只保留前 N 字节并标记 truncated，不能先完整缓冲再切片。
6. 下载、流式响应和导出路由完全跳过 body capture。
7. 增加大响应测试，验证 writer 内存占用不随响应体线性增长。

验收：

- 100 MiB 下载不会产生约 100 MiB 的额外日志 buffer；
- 普通 GET 不复制 response body；
- POST 审计日志仍保留受限、脱敏后的结果。
- 任意租户只能查询自己的操作日志。

### P0-4 固定可信代理与监听边界

在建立基线前先固定可信代理边界：

1. 生产 service 默认改为监听 `127.0.0.1:8080`，或由防火墙/安全组确保只有 nginx 能访问 Go 端口。
2. 若必须接受多级代理，显式配置可信代理网段；只在远端地址可信时读取代理头。
3. 增加直接访问 Go 端口并伪造 `X-Real-IP` 的部署测试，确保生产拓扑下无法绕过。

验收：

- 公网无法直接连接 Go 监听端口；
- 客户端自带 `X-Real-IP` / `X-Forwarded-For` 不会改变应用识别的来源 IP；
- 登录 IP 绑定与所有按 IP 限流使用同一可信来源。

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

### P0-6 多实例安全状态在 Redis 故障时失败关闭

现状：

- Redis 只在启动阶段强制可用。
- 运行期所有 cache 操作失败后都回落到进程内 `sync.Map`。
- 多实例下 session、权限版本、限流、配置失效和 export task 会分裂；readiness 虽失败，已进入实例的请求仍继续使用本机状态。

实施：

1. 把 session、权限、限流、导出任务等安全/协调状态与普通公开缓存分开。
2. `RequireRedisConfigured()` 为真时，安全状态的 Redis 错误必须失败关闭，不能读写本机 fallback。
3. 公开配置等允许降级的数据可使用有容量上限和淘汰策略的短 TTL L1。
4. Redis 故障立即使实例 readiness 失败，并确保负载均衡摘流；记录 error/fallback 指标。
5. 增加运行期断开 Redis 的多实例测试，覆盖权限撤销、登录、限流和任务状态。

验收：

- Redis 故障不会绕过权限/限流，也不会让已撤销 session 在单个实例继续有效；
- 恢复 Redis 后不会重新使用故障期间产生的不一致本机状态。

### P0-7 把生产索引创建移出服务启动

现状：

- 已安装应用在 HTTP 监听前同步执行 `EnsurePerfIndexes`。
- 它会枚举所有分表并串行执行 `CREATE INDEX`；大表可能等待 metadata lock 或超过 systemd 启动预算。
- 创建失败只写日志，服务最终可能在索引缺失状态下继续运行。

实施：

1. 将 DDL 放入显式、幂等、可观测的升级步骤；按 MySQL 版本配置 online DDL 策略。
2. 发布前展示待执行表、索引、预计锁影响和执行结果。
3. 应用启动只校验必要 schema version，不修改大表。
4. 为失败、超时和部分完成提供可重试状态，不以普通日志代替迁移结果。

验收：

- API 发布启动时间不受业务表大小和分表数影响；
- 索引迁移有独立状态、日志和失败告警；
- 未完成必要迁移时按明确策略拒绝切流，而不是静默带病运行。

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
4. 扩充敏感字段脱敏清单，覆盖 token、证书、AES key 和嵌套配置。

验收：

- 队列拥塞和 DB 故障有指标与告警；
- 安全审计日志满足明确的保留与可靠性要求；
- 日志高峰不明显抬高普通 API p99。

### P1-3 查询、缓存与只读副本细化

1. 平台 `PayWayGet` 和租户 `PayWayGet` 先收集 `pay_config_id`，用一次 `IN (?)` 查询配置并映射，删除逐行 `First`。
2. 平台租户列表为 `tactics=1` 分表租户逐个 COUNT；应提供批量汇总来源、缓存统计或明确限制分表租户列表统计成本。
3. `BumpBoot` 已有版本 key，继续 `SCAN boot:{tenant}:*` 属于重复失效；改为仅 bump，并依赖 TTL 清理旧版本。
4. boot key 中 host 需规范化并限制长度，避免异常 Host 制造高基数 key。
5. replica 探活不应在请求 goroutine 中持全局 mutex 等待 Ping；改为后台健康状态或 singleflight/atomic。
6. 对读副本延迟敏感的路径建立清单，写后立即读、余额、支付、登录和权限始终走主库。
7. 安装向导当前提示“导出读走从库”，但导出仍使用原列表查询路径；在真正接入副本前修正文案，避免错误的运维预期。
8. 用生产数据 `EXPLAIN ANALYZE` 验证当前首批索引；启动自动 DDL 只作为兼容手段，生产升级仍应使用可审计迁移。

### P1-4 静态、上传与 CDN 完成态

1. 多实例上线前将本地上传迁到 OSS，或验证所有实例共享同一挂载及权限。
2. 带内容 hash 的 SPA 文件设置 `immutable`；HTML 保持短缓存或 no-cache。
3. 上传使用独立 location、体积和超时预算，不扩大普通 API 预算。
4. CDN 只缓存公开、无用户态内容；缓存 key 必须包含 tenant、host、终端和相关 query。

### P1-5 运行期正确性与故障边界

1. 限流当前分开执行 `INCR` 和 `EXPIRE`；改为 Lua/事务原子操作，避免中间失败留下永久计数。
2. `/readyz` 使用带短 deadline 的 `PingContext`；MySQL DSN 增加 dial/read/write timeout。
3. replica 不能只检查 TCP Ping：
   - 启动失败后应重试绑定；
   - 检查 schema version、复制状态和可接受 lag；
   - 可安全回退的读在 query error 时切回主库；
   - 暴露 replica health/lag 指标。
4. boot payload 包含请求生成的资源 origin，但 key 只有 host 没有 scheme；使用规范化外部 origin/CDN，或把校验后的 scheme 纳入 key，避免 HTTP 缓存污染 HTTPS。
5. `ArticleCateUpdateStatus` 成功后补 `invalidatePublic(c, "cate")`，不能只等待 TTL。
6. `schemacache.HasColumn` 的正负结果不能永久缓存且只按 table/column 区分；加入数据库身份/schema version 和 TTL，迁移后主动失效。
7. 传播 `http.MaxBytesError` 并返回 413；普通 JSON 接口使用远小于上传的 route-specific body limit。

验收：

- 依赖黑洞、复制中断、在线 schema 变更和超大请求都有确定的超时、错误码、回退与指标；
- HTTP/HTTPS boot 资源 origin 不互相污染；
- 分类状态变更在主动失效后立即对公网可见。

### P2 后续容量演进

- 工作台统计在数据证明需要时迁移到分钟级汇总或事件聚合。
- 对 `%keyword%` 查询先验证产品能否改为前缀检索，再决定全文索引/搜索服务。
- cron 全租户扫描和 `information_schema` 探测按指标决定是否分批和缓存。
- 只有在缓存、索引、查询和读副本优化后主库仍长期达到瓶颈，才评估独立库、哈希分片、TiDB/Vitess。
- `tactics=1` 的租户分表是兼容/隔离能力，不作为默认性能方案。

## 4. 实施顺序

1. 修复权限 DB 错误 fail-open。
2. 修复 tenant task 轮询，再把导出改为独立 worker + 游标读取 + 共享/对象存储 + 鉴权下载和文件清理。
3. 给操作日志增加 tenant ID、递归脱敏，并移除 GET/下载/大响应全量缓冲。
4. 将 Go 监听限制在可信代理边界内。
5. 多实例安全状态在 Redis 运行期故障时失败关闭。
6. 把生产索引 DDL 移出应用启动。
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
