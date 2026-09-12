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

func TestFinishTaskOnlyFirstTerminalStateWins(t *testing.T) {
	id := newTaskID()
	saveTask(Task{ID: id, Status: statusPending, AdminID: 1})
	t.Cleanup(func() {
		cache.Del(taskCacheKey(id))
		cache.Del("export_finish_" + id)
	})
	if !finishTask(Task{ID: id, Status: statusFailed, Msg: "timeout", AdminID: 1}) {
		t.Fatal("first terminal state must win")
	}
	if finishTask(Task{ID: id, Status: statusReady, File: "late", AdminID: 1}) {
		t.Fatal("late completion must not overwrite timeout")
	}
	got, ok := loadTask(id)
	if !ok || got.Status != statusFailed || got.Msg != "timeout" {
		t.Fatalf("task %+v ok=%v", got, ok)
	}
}

func TestExportAdminInfoDropsSessionSecrets(t *testing.T) {
	got := exportAdminInfo(map[string]any{
		"admin_id": 1, "root": 1, "role_id": []int{2},
		"token": "secret", "login_ip": "127.0.0.1", "expire_time": int64(9),
	})
	if got["admin_id"] != 1 || got["root"] != 1 {
		t.Fatalf("required auth context missing: %#v", got)
	}
	for _, key := range []string{"token", "login_ip", "expire_time"} {
		if _, ok := got[key]; ok {
			t.Fatalf("job persisted %s: %#v", key, got)
		}
	}
}

func TestTenantLeaseSerializesJobs(t *testing.T) {
	first := Job{ID: newTaskID(), TenantID: 42}
	release, ok := acquireTenantLease(first)
	if !ok {
		t.Fatal("first lease")
	}
	t.Cleanup(release)
	if _, ok := acquireTenantLease(Job{ID: newTaskID(), TenantID: 42}); ok {
		t.Fatal("same tenant acquired concurrent lease")
	}
	release()
	release2, ok := acquireTenantLease(Job{ID: newTaskID(), TenantID: 42})
	if !ok {
		t.Fatal("lease was not released")
	}
	release2()
}
