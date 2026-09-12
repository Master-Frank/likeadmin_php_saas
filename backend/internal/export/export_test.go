package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

func TestLookupLogFields(t *testing.T) {
	spec := Lookup("setting.system.log", "lists")
	if spec.FileName != "系统日志" || len(spec.Fields) != 9 {
		t.Fatalf("%+v", spec)
	}
	if spec.Fields[0].Title != "记录ID" || spec.Fields[1].Key != "action" {
		t.Fatalf("fields %+v", spec.Fields)
	}
}

func TestToRecordsUsesChineseHeaders(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "action": " 查看系统日志列表", "extra": "drop"},
	}
	rec := toRecords(rows, []Field{{Key: "id", Title: "记录ID"}, {Key: "action", Title: "操作"}})
	if len(rec) != 2 || rec[0][0] != "记录ID" || rec[0][1] != "操作" {
		t.Fatalf("%v", rec)
	}
	if rec[1][0] != "1" || rec[1][1] != " 查看系统日志列表" {
		t.Fatalf("row %v", rec[1])
	}
}

func TestLookupCompactName(t *testing.T) {
	spec := Lookup("tenant.tenantadmin", "lists")
	if spec.FileName != "租户用户列表" {
		t.Fatalf("%+v", spec)
	}
}

func TestFormatCellEnums(t *testing.T) {
	if formatCell("channel", 1) != "微信小程序" {
		t.Fatalf("channel %s", formatCell("channel", 1))
	}
	if formatCell("disable", 0) != "0" || formatCell("disable", 1) != "1" {
		t.Fatalf("disable %s %s", formatCell("disable", 0), formatCell("disable", 1))
	}
	if formatCell("pay_status_text", 1) != "已支付" {
		t.Fatalf("pay %s", formatCell("pay_status_text", 1))
	}
	if formatCell("pay_status_text", "已支付") != "已支付" {
		t.Fatalf("already text %s", formatCell("pay_status_text", "已支付"))
	}
	rec := toRecords([]map[string]any{{"channel": 2, "disable": 1}}, []Field{{Key: "channel", Title: "注册来源"}, {Key: "disable", Title: "是否禁用"}})
	if len(rec) != 2 || rec[1][0] != "微信公众号" || rec[1][1] != "1" {
		t.Fatalf("%v", rec)
	}
}

func TestExcelLongNumericTab(t *testing.T) {
	if !isLongNumeric("123456789012") {
		t.Fatal("12-digit should be long")
	}
	if isLongNumeric("12345678901") {
		t.Fatal("11-digit should not be long")
	}
	out := applyExcelLongNumbers([][]string{{"id"}, {"123456789012"}, {"abc"}})
	if out[1][0] != "123456789012\t" {
		t.Fatalf("tab suffix %q", out[1][0])
	}
	if out[2][0] != "abc" {
		t.Fatalf("text %q", out[2][0])
	}
}

func TestExportRangeError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSize, oldMax := config.C.Project.Lists.PageSize, config.C.Project.Lists.PageSizeMax
	config.C.Project.Lists.PageSize = 25
	config.C.Project.Lists.PageSizeMax = 25000
	t.Cleanup(func() {
		config.C.Project.Lists.PageSize = oldSize
		config.C.Project.Lists.PageSizeMax = oldMax
	})

	ctx := func(raw string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/lists"+raw, nil)
		return c
	}

	if msg := exportRangeError(ctx("?export=2&page_start=999&page_end=999"), 10); msg != "第999页到第999页没有数据，无法导出" {
		t.Fatalf("paged empty %q", msg)
	}
	if msg := exportRangeError(ctx("?export=2&page_type=0"), 0); msg != "没有数据,无法导出" {
		t.Fatalf("unpaged empty %q", msg)
	}
	if msg := exportRangeError(ctx("?export=2&page_type="), 0); msg != "没有数据,无法导出" {
		t.Fatalf("empty page_type is unpaged %q", msg)
	}
	if msg := exportRangeError(ctx("?export=2&page_start=1&page_end=1"), 10); msg != "" {
		t.Fatalf("has data %q", msg)
	}
}

func TestExportWindowLimitError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.Project.Lists
	config.C.Project.Lists.PageSize = 25
	config.C.Project.Lists.PageSizeMax = 25000
	config.C.Project.Lists.ExportMaxRows = 10000
	config.C.Project.Lists.ExportMaxPages = 20
	t.Cleanup(func() { config.C.Project.Lists = old })

	ctx := func(raw string) *gin.Context {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/lists"+raw, nil)
		return c
	}
	if msg := exportWindowLimitError(ctx("?export=2&page_start=1&page_end=200&page_size=25000")); msg == "" {
		t.Fatal("200*25000 must be rejected")
	}
	if msg := exportWindowLimitError(ctx("?export=2&page_start=1&page_end=200&page_size=10")); msg == "" {
		t.Fatal("200 pages must be rejected")
	}
	if msg := exportWindowLimitError(ctx("?export=2&page_start=1&page_end=4&page_size=10")); msg != "" {
		t.Fatalf("small window %q", msg)
	}
}

func TestMaybeIgnoresBodyExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/setting.system.log/lists?page_size=1", bytes.NewBufferString(`{"export":2,"file_name":"hack"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	ctxutil.Set(c, &ctxutil.RequestMeta{
		Controller: "setting.system.log", Action: "lists", App: "platformapi", AdminID: 7,
	})
	if Maybe(c, "export", []map[string]any{{"id": 1}}) {
		t.Fatal("body export=2 must be ignored")
	}
}

func TestMaybeExportURLUsesAppPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenantapi/user.user/lists?export=2&page_start=1&page_end=1", nil)
	c.Request.Host = "pair1.likeadmin.test"
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "user.user", Action: "lists", App: "tenantapi", AdminID: 7, TenantID: 3})
	if !Maybe(c, "用户列表", []map[string]any{{"id": 1, "account": "a"}}) {
		t.Fatal("export=2")
	}
	var env struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	url, _ := env.Data["url"].(string)
	if env.Code != 1 || !strings.Contains(url, "/tenantapi/download/export?file=") {
		t.Fatalf("tenant export url must use tenantapi: code=%d url=%s body=%s", env.Code, url, w.Body.String())
	}
	if !strings.Contains(url, "sig=") || !strings.Contains(url, "exp=") {
		t.Fatalf("download url must be signed: %s", url)
	}
	if env.Data["status"] != "ready" || env.Data["task_id"] == "" {
		t.Fatalf("sync export must return ready task: %s", w.Body.String())
	}
	if strings.Contains(url, "/platformapi/") {
		t.Fatalf("platform prefix leaked: %s", url)
	}
}

func TestMaybeRejectsUnsupportedExport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/auth.role/lists?export=2", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "auth.role", Action: "lists", App: "platformapi"})
	if !Maybe(c, "角色表", []map[string]any{{"name": "r"}}) {
		t.Fatal("export=2 should be handled")
	}
	if !strings.Contains(w.Body.String(), "该列表不支持导出") {
		t.Fatalf("body %s", w.Body.String())
	}
}

func TestMaybeQueryFileName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/lists?export=1&file_name=自定义导出", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "setting.system.log", Action: "lists"})
	c.Set("likeadmin.export_count", int64(3))
	if !Maybe(c, "export", []map[string]any{{"id": 1}}) {
		t.Fatal("export=1")
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data["file_name"] != "自定义导出" {
		t.Fatalf("file_name %v", env.Data["file_name"])
	}
}

func TestMaybeFileNameZeroUsesDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/lists?export=1&file_name=0", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "setting.system.log", Action: "lists"})
	c.Set("likeadmin.export_count", int64(1))
	if !Maybe(c, "export", []map[string]any{{"id": 1}}) {
		t.Fatal("export=1")
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data["file_name"] != "系统日志" {
		t.Fatalf("file_name=0 must fall back like PHP ?:, got %v", env.Data["file_name"])
	}
}

func TestWriteXLSXZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.xlsx")
	if err := writeXLSX(path, [][]string{{"记录ID", "操作"}, {"1", "查看"}}); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	seen := map[string]bool{}
	var sheet string
	for _, f := range r.File {
		seen[f.Name] = true
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			buf := make([]byte, 4096)
			n, _ := rc.Read(buf)
			_ = rc.Close()
			sheet = string(buf[:n])
		}
	}
	for _, name := range []string{"[Content_Types].xml", "xl/workbook.xml", "xl/worksheets/sheet1.xml"} {
		if !seen[name] {
			t.Fatalf("missing %s", name)
		}
	}
	if !strings.Contains(sheet, "记录ID") || !strings.Contains(sheet, "查看") {
		t.Fatalf("sheet %s", sheet)
	}
}

func TestServeTaskAndSyncReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldApp, oldProj := config.C.App, config.C.Project
	t.Cleanup(func() {
		config.C.App, config.C.Project = oldApp, oldProj
	})
	config.C.App.MultiInstance = false
	config.C.Project.ExportAsync = false
	t.Setenv("LIKEADMIN_EXPORT_ASYNC", "")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/auth.admin/lists?export=2&page_start=1&page_end=1", nil)
	c.Request.Host = "pair1.likeadmin.test"
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "setting.system.log", Action: "lists", App: "platformapi"})
	if !Maybe(c, "系统日志", []map[string]any{{"id": 1}}) {
		t.Fatal("export=2")
	}
	var env struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	taskID, _ := env.Data["task_id"].(string)
	if env.Code != 1 || taskID == "" || env.Data["status"] != "ready" {
		t.Fatalf("body %s", w.Body.String())
	}
	saved, ok := loadTask(taskID)
	if !ok || saved.AdminID != 7 {
		t.Fatalf("saved task %+v ok=%v", saved, ok)
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/platformapi/download/export?task="+taskID, nil)
	ctxutil.Set(c2, &ctxutil.RequestMeta{AdminID: 7})
	Serve(c2)
	if !strings.Contains(w2.Body.String(), `"status":"ready"`) {
		t.Fatalf("poll %s", w2.Body.String())
	}

	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/platformapi/download/export?task=missing", nil)
	Serve(c3)
	if !strings.Contains(w3.Body.String(), "导出任务不存在") {
		t.Fatalf("missing %s", w3.Body.String())
	}
}

func TestSaveExportOutsidePublicUploads(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "public")
	if err := os.MkdirAll(pub, 0o755); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.PublicDir
	t.Cleanup(func() { config.C.App.PublicDir = old })
	config.C.App.PublicDir = pub
	key, err := SaveXLSX("demo", []map[string]any{{"id": 1}}, []Field{{Key: "id", Title: "ID"}})
	if err != nil {
		t.Fatal(err)
	}
	var info fileInfo
	if !cache.GetJSON("export_file_"+key, &info) {
		t.Fatal("file meta missing")
	}
	t.Cleanup(func() { cache.Del("export_file_" + key) })
	if strings.Contains(info.Name, "uploads") || strings.Contains(exportRoot(), "uploads") {
		t.Fatalf("must not write under public uploads: %s", exportRoot())
	}
	want := filepath.Join(dir, "runtime", "export")
	if !strings.HasPrefix(exportRoot(), want) {
		t.Fatalf("export root %s want prefix %s", exportRoot(), want)
	}
	if info.Rel != info.Name || info.Name == "" {
		t.Fatalf("relative name %+v", info)
	}
}

func TestTaskOwnerMismatchHidden(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := newTaskID()
	saveTask(Task{ID: id, Status: statusReady, AdminID: 9})
	t.Cleanup(func() { cache.Del(taskCacheKey(id)) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/download/export?task="+id, nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{AdminID: 1})
	serveTask(c, id)
	if !strings.Contains(w.Body.String(), "导出任务不存在") {
		t.Fatalf("%s", w.Body.String())
	}
}

func TestTaskWithoutOwnerIsNotPollable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	id := newTaskID()
	saveTask(Task{ID: id, Status: statusReady})
	t.Cleanup(func() { cache.Del(taskCacheKey(id)) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/download/export?task="+id, nil)
	serveTask(c, id)
	if !strings.Contains(w.Body.String(), "导出任务不存在") {
		t.Fatalf("%s", w.Body.String())
	}
}

func TestNewTaskIDRandom(t *testing.T) {
	a, b := newTaskID(), newTaskID()
	if a == b || len(a) < 16 {
		t.Fatalf("%s %s", a, b)
	}
}

func TestMaybeExportPreviewPageEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.Project.Lists
	config.C.Project.Lists.ExportMaxPages = 20
	t.Cleanup(func() { config.C.Project.Lists = old })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/lists?export=1", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Controller: "setting.system.log", Action: "lists"})
	c.Set("likeadmin.export_count", int64(10000))
	if !Maybe(c, "export", []map[string]any{{"id": 1}}) {
		t.Fatal("export=1")
	}
	var env struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Data["page_end"] != float64(20) {
		t.Fatalf("page_end %v", env.Data["page_end"])
	}
}

func TestServeKeepsKeyWhenFileMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := withExportRoot(t)
	key := randomHex(8)
	owner := TaskOwner{AdminID: 2, TenantID: 4}
	cache.Set("export_file_"+key, fileInfo{Src: dir + string(os.PathSeparator), Name: "gone.xlsx", AdminID: owner.AdminID, TenantID: owner.TenantID}, time.Hour)
	t.Cleanup(func() { cache.Del("export_file_" + key) })
	exp := time.Now().Add(time.Minute).Unix()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenantapi/download/export?file="+key+"&exp="+itoa64(exp)+"&sig="+signExportFile(key, owner, exp), nil)
	Serve(c)
	if !strings.Contains(w.Body.String(), "下载文件不存在") {
		t.Fatalf("body %s", w.Body.String())
	}
	var info fileInfo
	if !cache.GetJSON("export_file_"+key, &info) {
		t.Fatal("missing file must not consume the download key")
	}
}

func TestServeRejectsUnsignedDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := withExportRoot(t)
	name := "ok.xlsx"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("xlsx"), 0o644); err != nil {
		t.Fatal(err)
	}
	key := randomHex(8)
	cache.Set("export_file_"+key, fileInfo{Src: dir + string(os.PathSeparator), Name: name, AdminID: 9, TenantID: 3}, time.Hour)
	t.Cleanup(func() { cache.Del("export_file_" + key) })
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/download/export?file="+key, nil)
	Serve(c)
	if !strings.Contains(w.Body.String(), "下载文件不存在") {
		t.Fatalf("body %s", w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
		t.Fatal("unsigned request must not delete the file")
	}
}

func TestServeDeletesFileAfterDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := withExportRoot(t)
	name := "ok.xlsx"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("xlsx-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	key := randomHex(8)
	owner := TaskOwner{AdminID: 1}
	cache.Set("export_file_"+key, fileInfo{Src: dir + string(os.PathSeparator), Name: name, Download: "demo.xlsx", AdminID: owner.AdminID}, time.Hour)
	exp := time.Now().Add(time.Minute).Unix()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/download/export?file="+key+"&exp="+itoa64(exp)+"&sig="+signExportFile(key, owner, exp), nil)
	Serve(c)
	if w.Body.String() != "xlsx-bytes" {
		t.Fatalf("body %q", w.Body.String())
	}
	if _, ok := cache.Get("export_file_" + key); ok {
		t.Fatal("consumed key should be gone")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("downloaded file should be removed")
	}
}

func TestCleanOldExports(t *testing.T) {
	dir := withExportRoot(t)
	keep := filepath.Join(dir, "keep.xlsx")
	drop := filepath.Join(dir, "old.xlsx")
	if err := os.WriteFile(keep, []byte("k"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(drop, []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldTime := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(drop, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	cleanOldExports(30 * time.Minute)
	if _, err := os.Stat(drop); !os.IsNotExist(err) {
		t.Fatal("stale export should be removed")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("fresh export should stay")
	}
}

func withExportRoot(t *testing.T) string {
	t.Helper()
	old := config.C.App.PublicDir
	root := t.TempDir()
	config.C.App.PublicDir = filepath.Join(root, "public")
	t.Cleanup(func() { config.C.App.PublicDir = old })
	dir := exportRoot()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}
