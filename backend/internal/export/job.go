package export

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/metrics"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

const (
	jobQueueKey  = "export_jobs"
	jobLeasePref = "export_lease_"
	jobLeaseTTL  = 2 * time.Minute
	jobMaxAge    = 10 * time.Minute
	maxFileBytes = 50 << 20
	workerN      = 2
)

type Job struct {
	ID         string         `json:"id"`
	App        string         `json:"app"`
	Controller string         `json:"controller"`
	Action     string         `json:"action"`
	RawQuery   string         `json:"raw_query"`
	Host       string         `json:"host"`
	Scheme     string         `json:"scheme"`
	AdminID    uint           `json:"admin_id"`
	TenantID   uint           `json:"tenant_id"`
	TenantSN   string         `json:"tenant_sn"`
	Tactics    int            `json:"tactics"`
	AdminInfo  map[string]any `json:"admin_info,omitempty"`
	Domain     string         `json:"domain,omitempty"`
	Created    int64          `json:"created"`
	Attempts   int            `json:"attempts,omitempty"`
}

type HandlerLookup func(app, controller, action string) gin.HandlerFunc

var (
	lookupHandler HandlerLookup
	workersOnce   sync.Once
	stopping      atomic.Bool
	inFlight      atomic.Int64
	memJobs       chan string
	tenantLocks   sync.Map // uint -> *sync.Mutex
)

func SetHandlerLookup(fn HandlerLookup) { lookupHandler = fn }

func WorkerInFlight() int64 { return inFlight.Load() }

// EnqueueFromRequest captures export=2 conditions and returns a task ID without
// running the list query. ParseGET callers should stop when this returns true.
func EnqueueFromRequest(c *gin.Context) bool {
	if c == nil || c.GetBool("likeadmin.export_worker") {
		return false
	}
	if httpx.QueryInt(c, "export") != 2 || !config.ExportAsyncEnabled() {
		return false
	}
	meta := ctxutil.Get(c)
	if meta.Controller == "" || meta.Action == "" {
		return false
	}
	if lookupHandler == nil || lookupHandler(meta.App, meta.Controller, meta.Action) == nil {
		return false
	}
	owner := exportOwner(c)
	id := newTaskID()
	job := Job{
		ID: id, App: meta.App, Controller: meta.Controller, Action: meta.Action,
		RawQuery: c.Request.URL.RawQuery, Host: ctxutil.Host(c), Scheme: ctxutil.Scheme(c),
		Domain: ctxutil.Domain(c), AdminID: owner.AdminID, TenantID: owner.TenantID, TenantSN: meta.TenantSN,
		Tactics: meta.Tactics, AdminInfo: meta.AdminInfo, Created: time.Now().Unix(),
	}
	saveTask(Task{ID: id, Status: statusPending, AdminID: owner.AdminID, TenantID: owner.TenantID})
	metrics.AddExport("pending")
	cache.Set(jobCacheKey(id), job, taskTTL)
	if !pushJob(id) {
		saveTask(Task{ID: id, Status: statusFailed, Msg: "导出队列不可用", AdminID: owner.AdminID, TenantID: owner.TenantID})
		metrics.AddExport(statusFailed)
		response.Fail(c, "服务繁忙，请稍后再试")
		return true
	}
	response.Data(c, gin.H{"task_id": id, "status": statusPending})
	return true
}

func jobCacheKey(id string) string { return "export_job_" + id }

func pushJob(id string) bool {
	if cache.ListPush(jobQueueKey, id) {
		return true
	}
	if config.RequireRedisConfigured() {
		return false
	}
	if memJobs == nil {
		memJobs = make(chan string, 64)
	}
	select {
	case memJobs <- id:
		return true
	default:
		return false
	}
}

func popJob() (string, bool) {
	if id, ok := cache.ListPop(jobQueueKey, 2*time.Second); ok {
		return id, true
	}
	if memJobs == nil {
		return "", false
	}
	select {
	case id := <-memJobs:
		return id, true
	case <-time.After(200 * time.Millisecond):
		return "", false
	}
}

func Start() { startWorkers() }

func startWorkers() {
	workersOnce.Do(func() {
		if memJobs == nil {
			memJobs = make(chan string, 64)
		}
		for i := 0; i < workerN; i++ {
			go workerLoop()
		}
		go recoverLoop()
	})
}

func StopWorkers() {
	stopping.Store(true)
	deadline := time.Now().Add(10 * time.Second)
	for inFlight.Load() > 0 && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
}

func workerLoop() {
	for !stopping.Load() {
		id, ok := popJob()
		if !ok || id == "" {
			continue
		}
		runQueuedJob(id)
	}
}

func runQueuedJob(id string) {
	t, ok := loadTask(id)
	if !ok || t.Status != statusPending {
		return
	}
	if !cache.SetNX(jobLeasePref+id, config.InstanceID(), jobLeaseTTL) {
		return
	}
	defer cache.Del(jobLeasePref + id)
	var job Job
	if !cache.GetJSON(jobCacheKey(id), &job) || job.ID == "" {
		return
	}
	if time.Since(time.Unix(job.Created, 0)) > jobMaxAge {
		failJob(job, "导出任务超时")
		return
	}
	unlock := lockTenant(job.TenantID)
	defer unlock()
	inFlight.Add(1)
	metrics.SetExportInFlight(inFlight.Load())
	defer func() {
		inFlight.Add(-1)
		metrics.SetExportInFlight(inFlight.Load())
		if rec := recover(); rec != nil {
			failJob(job, "导出失败")
		}
	}()
	if lookupHandler == nil {
		failJob(job, "导出任务无法执行")
		return
	}
	h := lookupHandler(job.App, job.Controller, job.Action)
	if h == nil {
		failJob(job, "该列表不支持导出")
		return
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	path := "/" + job.App + "/" + job.Controller + "/" + job.Action
	if job.RawQuery != "" {
		path += "?" + job.RawQuery
	}
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Request.Host = job.Host
	if job.Scheme != "" {
		c.Request.Header.Set("X-Forwarded-Proto", job.Scheme)
	}
	c.Set("likeadmin.export_worker", true)
	c.Set("likeadmin.export_task_id", job.ID)
	if job.Domain != "" {
		c.Set("likeadmin.export_domain", job.Domain)
	}
	ctxutil.Set(c, &ctxutil.RequestMeta{
		App: job.App, Controller: job.Controller, Action: job.Action,
		AdminID: job.AdminID, TenantID: job.TenantID, TenantSN: job.TenantSN,
		Tactics: job.Tactics, AdminInfo: job.AdminInfo,
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		h(c)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Minute):
		failJob(job, "导出执行超时")
		return
	}
	got, ok := loadTask(job.ID)
	if !ok || got.Status == statusPending {
		msg := "导出失败"
		var env struct {
			Msg string `json:"msg"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Msg != "" {
			msg = env.Msg
		}
		failJob(job, msg)
	}
}

func failJob(job Job, msg string) {
	saveTask(Task{ID: job.ID, Status: statusFailed, Msg: msg, AdminID: job.AdminID, TenantID: job.TenantID})
	metrics.AddExport(statusFailed)
}

func lockTenant(tid uint) func() {
	v, _ := tenantLocks.LoadOrStore(tid, &sync.Mutex{})
	mu := v.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func recoverLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if stopping.Load() {
			return
		}
		recoverPendingJobs()
	}
}

func recoverPendingJobs() {
	for _, key := range cache.KeysPrefix("export_job_") {
		id := strings.TrimPrefix(key, "export_job_")
		if id == "" {
			continue
		}
		t, ok := loadTask(id)
		if !ok || t.Status != statusPending {
			continue
		}
		if _, leased := cache.Get(jobLeasePref + id); leased {
			continue
		}
		var job Job
		if !cache.GetJSON(jobCacheKey(id), &job) || job.ID == "" {
			continue
		}
		if time.Since(time.Unix(job.Created, 0)) > jobMaxAge || job.Attempts >= 3 {
			failJob(job, "导出任务超时")
			continue
		}
		job.Attempts++
		cache.Set(jobCacheKey(id), job, taskTTL)
		_ = pushJob(id)
	}
}
