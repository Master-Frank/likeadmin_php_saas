package export

import (
	"fmt"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/metrics"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	statusPending = "pending"
	statusReady   = "ready"
	statusFailed  = "failed"
	taskTTL       = 30 * time.Minute
)

type Task struct {
	ID     string `json:"task_id"`
	Status string `json:"status"`
	URL    string `json:"url,omitempty"`
	File   string `json:"file,omitempty"`
	Msg    string `json:"msg,omitempty"`
}

func taskCacheKey(id string) string { return "export_task_" + id }

func newTaskID() string {
	return util.MD5(fmt.Sprintf("export-task-%d", time.Now().UnixNano()))
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

func downloadURL(domain, fileKey string) string {
	return domain + "/platformapi/download/export?file=" + fileKey
}

func newReadyTask(domain, fileKey string) Task {
	t := Task{
		ID: newTaskID(), Status: statusReady,
		URL: downloadURL(domain, fileKey), File: fileKey,
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

func runExportTask(id, domain, fileName string, rows any, fields []Field) {
	key, err := SaveXLSX(fileName, rows, fields)
	if err != nil {
		saveTask(Task{ID: id, Status: statusFailed, Msg: err.Error()})
		metrics.AddExport(statusFailed)
		return
	}
	saveTask(Task{ID: id, Status: statusReady, URL: downloadURL(domain, key), File: key})
	metrics.AddExport(statusReady)
}

func serveTask(c *gin.Context, id string) {
	t, ok := loadTask(id)
	if !ok {
		response.Fail(c, "导出任务不存在")
		return
	}
	response.Data(c, taskPayload(t))
}
