package metrics

import (
	"database/sql"
	"fmt"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"likeadmin/backend/internal/config"
)

var httpBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

var (
	httpRequests   atomic.Int64
	httpInFlight   atomic.Int64
	httpDurCount   atomic.Int64
	httpDurSumMs   atomic.Int64
	httpDurBuckets [12]atomic.Int64
	sqlQueries     atomic.Int64
	sqlDurCount    atomic.Int64
	sqlDurSumMs    atomic.Int64
	sqlDurBuckets  [12]atomic.Int64
	exportPending  atomic.Int64
	exportReady    atomic.Int64
	exportFailed   atomic.Int64
	exportInFlight atomic.Int64
	oplogQueued    atomic.Int64
	oplogDropped   atomic.Int64
	oplogWritten   atomic.Int64
	redisErrors    atomic.Int64
	redisHits      atomic.Int64
	redisMisses    atomic.Int64
	redisFallbacks atomic.Int64
	replicaUp      atomic.Int64
	replicaLagMs   atomic.Int64
	sqlDB          atomic.Pointer[sql.DB]
)

func AddHTTP() { httpRequests.Add(1) }

func AddSQL() { sqlQueries.Add(1) }

func AddRedisError() { redisErrors.Add(1) }

func AddRedisHit() { redisHits.Add(1) }

func AddRedisMiss() { redisMisses.Add(1) }

func AddRedisFallback() { redisFallbacks.Add(1) }

func AddOplogQueued() { oplogQueued.Add(1) }

func AddOplogDropped() { oplogDropped.Add(1) }

func AddOplogWritten() { oplogWritten.Add(1) }

func SetExportInFlight(n int64) { exportInFlight.Store(n) }

func SetReplicaUp(up bool) {
	if up {
		replicaUp.Store(1)
		return
	}
	replicaUp.Store(0)
}

func SetReplicaLag(d time.Duration) {
	replicaLagMs.Store(d.Milliseconds())
}

func SetSQLDB(db *sql.DB) { sqlDB.Store(db) }

func HTTPInFlightAdd(n int64) { httpInFlight.Add(n) }

func ObserveHTTP(d time.Duration) {
	observe(&httpDurCount, &httpDurSumMs, httpDurBuckets[:], d)
}

func ObserveSQL(d time.Duration) {
	observe(&sqlDurCount, &sqlDurSumMs, sqlDurBuckets[:], d)
}

func observe(count, sumMs *atomic.Int64, buckets []atomic.Int64, d time.Duration) {
	sec := d.Seconds()
	count.Add(1)
	sumMs.Add(d.Milliseconds())
	placed := false
	for i, le := range httpBuckets {
		if sec <= le {
			buckets[i].Add(1)
			placed = true
			break
		}
	}
	if !placed {
		buckets[len(httpBuckets)].Add(1)
	}
}

func AddExport(status string) {
	switch status {
	case "pending":
		exportPending.Add(1)
	case "failed":
		exportFailed.Add(1)
	default:
		exportReady.Add(1)
	}
}

func WritePrometheus(w http.ResponseWriter) {
	id := config.InstanceID()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# TYPE likeadmin_http_requests_total counter\n")
	fmt.Fprintf(w, "likeadmin_http_requests_total{instance=%q} %d\n", id, httpRequests.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_http_in_flight gauge\n")
	fmt.Fprintf(w, "likeadmin_http_in_flight{instance=%q} %d\n", id, httpInFlight.Load())
	writeHistogram(w, "likeadmin_http_request_duration_seconds", id, httpDurCount.Load(), float64(httpDurSumMs.Load())/1000, httpDurBuckets[:])
	fmt.Fprintf(w, "# TYPE likeadmin_sql_queries_total counter\n")
	fmt.Fprintf(w, "likeadmin_sql_queries_total{instance=%q} %d\n", id, sqlQueries.Load())
	writeHistogram(w, "likeadmin_sql_query_duration_seconds", id, sqlDurCount.Load(), float64(sqlDurSumMs.Load())/1000, sqlDurBuckets[:])
	fmt.Fprintf(w, "# TYPE likeadmin_export_tasks_total counter\n")
	fmt.Fprintf(w, "likeadmin_export_tasks_total{instance=%q,status=%q} %d\n", id, "pending", exportPending.Load())
	fmt.Fprintf(w, "likeadmin_export_tasks_total{instance=%q,status=%q} %d\n", id, "ready", exportReady.Load())
	fmt.Fprintf(w, "likeadmin_export_tasks_total{instance=%q,status=%q} %d\n", id, "failed", exportFailed.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_export_in_flight gauge\n")
	fmt.Fprintf(w, "likeadmin_export_in_flight{instance=%q} %d\n", id, exportInFlight.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_oplog_queued_total counter\n")
	fmt.Fprintf(w, "likeadmin_oplog_queued_total{instance=%q} %d\n", id, oplogQueued.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_oplog_dropped_total counter\n")
	fmt.Fprintf(w, "likeadmin_oplog_dropped_total{instance=%q} %d\n", id, oplogDropped.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_oplog_written_total counter\n")
	fmt.Fprintf(w, "likeadmin_oplog_written_total{instance=%q} %d\n", id, oplogWritten.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_redis_errors_total counter\n")
	fmt.Fprintf(w, "likeadmin_redis_errors_total{instance=%q} %d\n", id, redisErrors.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_redis_hits_total counter\n")
	fmt.Fprintf(w, "likeadmin_redis_hits_total{instance=%q} %d\n", id, redisHits.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_redis_misses_total counter\n")
	fmt.Fprintf(w, "likeadmin_redis_misses_total{instance=%q} %d\n", id, redisMisses.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_redis_fallbacks_total counter\n")
	fmt.Fprintf(w, "likeadmin_redis_fallbacks_total{instance=%q} %d\n", id, redisFallbacks.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_replica_up gauge\n")
	fmt.Fprintf(w, "likeadmin_replica_up{instance=%q} %d\n", id, replicaUp.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_replica_lag_seconds gauge\n")
	fmt.Fprintf(w, "likeadmin_replica_lag_seconds{instance=%q} %.3f\n", id, float64(replicaLagMs.Load())/1000)
	writeDBStats(w, id)
	writeRuntime(w, id)
}

func writeHistogram(w http.ResponseWriter, name, id string, count int64, sum float64, buckets []atomic.Int64) {
	fmt.Fprintf(w, "# TYPE %s histogram\n", name)
	var cum int64
	for i, le := range httpBuckets {
		cum += buckets[i].Load()
		fmt.Fprintf(w, "%s_bucket{instance=%q,le=\"%g\"} %d\n", name, id, le, cum)
	}
	cum += buckets[len(httpBuckets)].Load()
	fmt.Fprintf(w, "%s_bucket{instance=%q,le=\"+Inf\"} %d\n", name, id, cum)
	fmt.Fprintf(w, "%s_sum{instance=%q} %.6f\n", name, id, sum)
	fmt.Fprintf(w, "%s_count{instance=%q} %d\n", name, id, count)
}

func writeDBStats(w http.ResponseWriter, id string) {
	db := sqlDB.Load()
	if db == nil {
		return
	}
	st := db.Stats()
	fmt.Fprintf(w, "# TYPE likeadmin_db_open_connections gauge\n")
	fmt.Fprintf(w, "likeadmin_db_open_connections{instance=%q} %d\n", id, st.OpenConnections)
	fmt.Fprintf(w, "# TYPE likeadmin_db_in_use gauge\n")
	fmt.Fprintf(w, "likeadmin_db_in_use{instance=%q} %d\n", id, st.InUse)
	fmt.Fprintf(w, "# TYPE likeadmin_db_idle gauge\n")
	fmt.Fprintf(w, "likeadmin_db_idle{instance=%q} %d\n", id, st.Idle)
	fmt.Fprintf(w, "# TYPE likeadmin_db_wait_count_total counter\n")
	fmt.Fprintf(w, "likeadmin_db_wait_count_total{instance=%q} %d\n", id, st.WaitCount)
	fmt.Fprintf(w, "# TYPE likeadmin_db_wait_duration_seconds counter\n")
	fmt.Fprintf(w, "likeadmin_db_wait_duration_seconds{instance=%q} %.6f\n", id, st.WaitDuration.Seconds())
	fmt.Fprintf(w, "# TYPE likeadmin_db_max_open_connections gauge\n")
	fmt.Fprintf(w, "likeadmin_db_max_open_connections{instance=%q} %d\n", id, st.MaxOpenConnections)
}

func writeRuntime(w http.ResponseWriter, id string) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Fprintf(w, "# TYPE likeadmin_go_goroutines gauge\n")
	fmt.Fprintf(w, "likeadmin_go_goroutines{instance=%q} %d\n", id, runtime.NumGoroutine())
	fmt.Fprintf(w, "# TYPE likeadmin_go_heap_alloc_bytes gauge\n")
	fmt.Fprintf(w, "likeadmin_go_heap_alloc_bytes{instance=%q} %d\n", id, ms.HeapAlloc)
	fmt.Fprintf(w, "# TYPE likeadmin_go_heap_sys_bytes gauge\n")
	fmt.Fprintf(w, "likeadmin_go_heap_sys_bytes{instance=%q} %d\n", id, ms.HeapSys)
	fmt.Fprintf(w, "# TYPE likeadmin_go_alloc_bytes_total counter\n")
	fmt.Fprintf(w, "likeadmin_go_alloc_bytes_total{instance=%q} %d\n", id, ms.TotalAlloc)
	fmt.Fprintf(w, "# TYPE likeadmin_go_gc_pause_seconds gauge\n")
	fmt.Fprintf(w, "likeadmin_go_gc_pause_seconds{instance=%q} %.9f\n", id, time.Duration(ms.PauseNs[(ms.NumGC+255)%256]).Seconds())
}
