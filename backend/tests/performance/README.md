# Performance tests

These files are **how to measure** capacity. They are not capacity results.
Do not publish QPS, p95, or “X times faster than PHP” numbers unless they came
from a run of these scripts against a named commit, dataset, and machine.

`query_budget_test.go` only checks that request SQL stats attach and that export
caps are configured. It is not a load test.

## Prerequisites

- k6 (`https://k6.io`) or vegeta
- A running Go API (`LIKEADMIN_METRICS=1` so `127.0.0.1:9090/metrics` is up)
- Redis if the install is multi-instance
- Dataset restored from a dump, not generated ad-hoc during the run

## Dataset tiers (record these, do not invent them)

| Tier | Tenants | Hot tenant share | `user` rows | `article` rows | `operation_log` rows |
|------|---------|------------------|-------------|----------------|----------------------|
| small | fill in | fill in | fill in | fill in | fill in |
| medium | fill in | fill in | fill in | fill in | fill in |
| large | fill in | fill in | fill in | fill in | fill in |

Record MySQL/Redis version, CPU, memory, instance count, and git SHA in the
capacity report template in `/performance.md`.

## Scenarios

```bash
export BASE_URL=http://127.0.0.1:8080
export PLATFORM_TOKEN=...
export TENANT_TOKEN=...
export TENANT_HOST=tenant.example.test

k6 run backend/tests/performance/k6/boot.js
k6 run backend/tests/performance/k6/article.js
k6 run backend/tests/performance/k6/admin-lists.js
k6 run backend/tests/performance/k6/user-center.js
k6 run backend/tests/performance/k6/write-path.js
k6 run backend/tests/performance/k6/export.js
```

Run each scenario twice: cold cache (flush Redis + restart) and hot cache.
Repeat with Redis stopped (single-instance only) and with the replica stopped
when one is configured.

## Metrics to capture

From k6: RPS, error rate, p50/p95/p99.
From `/metrics`: in-flight HTTP, HTTP/SQL histograms, `sql.DB` wait, Go heap,
goroutines, export in-flight, oplog dropped/queued, replica lag.
From MySQL: `SHOW GLOBAL STATUS` / slow query log for the same window.

EXPLAIN ANALYZE of the first-batch indexes must be run on production-like data;
this repo cannot substitute that measurement.
