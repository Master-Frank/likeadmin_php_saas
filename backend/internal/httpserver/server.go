package httpserver

import (
	"context"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"likeadmin/backend/internal/export"
	"likeadmin/backend/internal/metrics"
	"likeadmin/backend/internal/middleware"
)

const (
	readHeaderTimeout  = 10 * time.Second
	readTimeout        = 30 * time.Second
	writeTimeout       = 30 * time.Second
	exportWriteTimeout = 120 * time.Second
	idleTimeout        = 60 * time.Second
	maxHeaderBytes     = 1 << 20
	shutdownWait       = 15 * time.Second
)

var (
	maxBodyBytes  int64 = 50 << 20
	jsonBodyBytes int64 = 1 << 20
)

// Run starts an http.Server with timeouts and SIGTERM graceful shutdown.
func Run(addr string, h http.Handler) error {
	startPprof()
	startMetrics()
	srv := &http.Server{
		Addr:              addr,
		Handler:           withLimits(MergeSlashes(h)),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("likeadmin-go listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-quit:
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownWait)
	defer cancel()
	err := srv.Shutdown(ctx)
	export.StopWorkers()
	middleware.DrainOplog()
	return err
}

func withLimits(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r != nil && r.Body != nil {
			limit := jsonBodyBytes
			if isUploadRequest(r) {
				limit = maxBodyBytes
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		if r != nil && isExportRequest(r) {
			rc := http.NewResponseController(w)
			_ = rc.SetWriteDeadline(time.Now().Add(exportWriteTimeout))
		}
		h.ServeHTTP(w, r)
	})
}

func isUploadRequest(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	return strings.Contains(strings.ToLower(r.URL.Path), "/upload/")
}

func isExportRequest(r *http.Request) bool {
	path := r.URL.Path
	if strings.Contains(path, "/download/export") {
		return true
	}
	q := r.URL.Query().Get("export")
	return q == "1" || q == "2"
}

func startPprof() {
	if os.Getenv("LIKEADMIN_PPROF") != "1" {
		return
	}
	go func() {
		log.Printf("pprof listening on 127.0.0.1:6060")
		if err := http.ListenAndServe("127.0.0.1:6060", nil); err != nil {
			log.Printf("pprof: %v", err)
		}
	}()
}

func startMetrics() {
	if os.Getenv("LIKEADMIN_METRICS") != "1" {
		return
	}
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
			metrics.WritePrometheus(w)
		})
		log.Printf("metrics listening on 127.0.0.1:9090")
		if err := http.ListenAndServe("127.0.0.1:9090", mux); err != nil {
			log.Printf("metrics: %v", err)
		}
	}()
}
