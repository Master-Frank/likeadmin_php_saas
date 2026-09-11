package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"likeadmin/backend/internal/config"
)

var (
	httpRequests  atomic.Int64
	sqlQueries    atomic.Int64
	exportPending atomic.Int64
	exportReady   atomic.Int64
	exportFailed  atomic.Int64
)

func AddHTTP() { httpRequests.Add(1) }

func AddSQL() { sqlQueries.Add(1) }

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
	fmt.Fprintf(w, "# TYPE likeadmin_sql_queries_total counter\n")
	fmt.Fprintf(w, "likeadmin_sql_queries_total{instance=%q} %d\n", id, sqlQueries.Load())
	fmt.Fprintf(w, "# TYPE likeadmin_export_tasks_total counter\n")
	fmt.Fprintf(w, "likeadmin_export_tasks_total{instance=%q,status=%q} %d\n", id, "pending", exportPending.Load())
	fmt.Fprintf(w, "likeadmin_export_tasks_total{instance=%q,status=%q} %d\n", id, "ready", exportReady.Load())
	fmt.Fprintf(w, "likeadmin_export_tasks_total{instance=%q,status=%q} %d\n", id, "failed", exportFailed.Load())
}
