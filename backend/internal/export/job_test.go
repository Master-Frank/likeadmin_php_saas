package export

import (
	"testing"
	"time"

	"likeadmin/backend/internal/cache"

	"github.com/gin-gonic/gin"
)

func TestRunQueuedJobReusesTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := withExportRoot(t)
	_ = dir
	SetHandlerLookup(func(app, controller, action string) gin.HandlerFunc {
		return func(c *gin.Context) {
			_ = Maybe(c, "系统日志", []map[string]any{{"id": 1, "action": "x"}})
		}
	})
	t.Cleanup(func() { SetHandlerLookup(nil) })

	id := newTaskID()
	owner := TaskOwner{AdminID: 8, TenantID: 2}
	saveTask(Task{ID: id, Status: statusPending, AdminID: owner.AdminID, TenantID: owner.TenantID})
	cache.Set(jobCacheKey(id), Job{
		ID: id, App: "platformapi", Controller: "setting.system.log", Action: "lists",
		RawQuery: "export=2", AdminID: owner.AdminID, TenantID: owner.TenantID, Created: time.Now().Unix(),
		Domain: "http://example.test",
	}, taskTTL)
	t.Cleanup(func() {
		cache.Del(taskCacheKey(id))
		cache.Del(jobCacheKey(id))
		cache.Del(jobLeasePref + id)
	})
	runQueuedJob(id)
	got, ok := loadTask(id)
	if !ok || got.Status != statusReady || got.File == "" {
		t.Fatalf("worker task %+v ok=%v", got, ok)
	}
	if got.ID != id {
		t.Fatalf("task id changed %s -> %s", id, got.ID)
	}
}

func TestRecoverPendingJobsRequeues(t *testing.T) {
	id := newTaskID()
	saveTask(Task{ID: id, Status: statusPending, AdminID: 1})
	cache.Set(jobCacheKey(id), Job{ID: id, Created: time.Now().Unix()}, taskTTL)
	t.Cleanup(func() {
		cache.Del(taskCacheKey(id))
		cache.Del(jobCacheKey(id))
	})
	if memJobs == nil {
		memJobs = make(chan string, 8)
	}
	recoverPendingJobs()
	select {
	case got := <-memJobs:
		if got != id {
			t.Fatalf("requeued %s", got)
		}
	default:
		t.Fatal("pending job without lease should be requeued")
	}
}
