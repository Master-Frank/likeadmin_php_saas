package export

import (
	"fmt"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/metrics"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

const (
	statusPending = "pending"
	statusReady   = "ready"
	statusFailed  = "failed"
	taskTTL       = 30 * time.Minute
	maxExportJobs = 2
)

type TaskOwner struct {
	AdminID  uint
	TenantID uint
}

type Task struct {
	ID       string `json:"task_id"`
	Status   string `json:"status"`
	URL      string `json:"url,omitempty"`
	File     string `json:"file,omitempty"`
	Msg      string `json:"msg,omitempty"`
	AdminID  uint   `json:"admin_id,omitempty"`
	TenantID uint   `json:"tenant_id,omitempty"`
}

func taskCacheKey(id string) string { return "export_task_" + id }

func newTaskID() string {
	return randomHex(16)
}

func saveTask(t Task) {
	if t.ID == "" {
		return
	}
	cache.Set(taskCacheKey(t.ID), t, taskTTL)
}

func loadTask(id string) (Task, bool) {
	var t Task
	if id == "" || !cache.GetJSON(taskCacheKey(id), &t) || t.ID == "" {
		return Task{}, false
	}
	return t, true
}

func downloadApp(app string) string {
	if app == "tenantapi" {
		return "tenantapi"
	}
	return "platformapi"
}

func downloadURL(app, domain, fileKey string, owner TaskOwner) string {
	exp := time.Now().Add(taskTTL).Unix()
	sig := signExportFile(fileKey, owner, exp)
	return fmt.Sprintf("%s/%s/download/export?file=%s&exp=%d&sig=%s",
		domain, downloadApp(app), fileKey, exp, sig)
}

func newReadyTask(app, domain, fileKey string, owner TaskOwner) Task {
	t := Task{
		ID: newTaskID(), Status: statusReady,
		URL: downloadURL(app, domain, fileKey, owner), File: fileKey,
		AdminID: owner.AdminID, TenantID: owner.TenantID,
	}
	saveTask(t)
	metrics.AddExport(statusReady)
	return t
}

func taskPayload(t Task) gin.H {
	out := gin.H{"task_id": t.ID, "status": t.Status}
	if t.URL != "" {
		out["url"] = t.URL
	}
	if t.Msg != "" {
		out["msg"] = t.Msg
	}
	return out
}

var exportSem = make(chan struct{}, maxExportJobs)

func runExportTask(id, app, domain, fileName string, rows any, fields []Field, owner TaskOwner) {
	defer func() {
		if rec := recover(); rec != nil {
			saveTask(Task{ID: id, Status: statusFailed, Msg: "导出失败", AdminID: owner.AdminID, TenantID: owner.TenantID})
			metrics.AddExport(statusFailed)
		}
	}()
	select {
	case exportSem <- struct{}{}:
		defer func() { <-exportSem }()
	default:
		saveTask(Task{ID: id, Status: statusFailed, Msg: "导出任务繁忙，请稍后重试", AdminID: owner.AdminID, TenantID: owner.TenantID})
		metrics.AddExport(statusFailed)
		return
	}
	key, err := saveOwnedXLSX(fileName, rows, fields, owner)
	if err != nil {
		saveTask(Task{ID: id, Status: statusFailed, Msg: err.Error(), AdminID: owner.AdminID, TenantID: owner.TenantID})
		metrics.AddExport(statusFailed)
		return
	}
	saveTask(Task{ID: id, Status: statusReady, URL: downloadURL(app, domain, key, owner), File: key, AdminID: owner.AdminID, TenantID: owner.TenantID})
	metrics.AddExport(statusReady)
}

func serveTask(c *gin.Context, id string) {
	t, ok := loadTask(id)
	if !ok || !taskOwnerOK(c, t) {
		response.Fail(c, "导出任务不存在")
		return
	}
	response.Data(c, taskPayload(t))
}

func taskOwnerOK(c *gin.Context, t Task) bool {
	if t.AdminID == 0 && t.TenantID == 0 {
		return true
	}
	meta := ctxutil.Get(c)
	if t.AdminID != 0 && meta.AdminID != t.AdminID {
		return false
	}
	if t.TenantID != 0 && meta.TenantID != t.TenantID {
		return false
	}
	return true
}
