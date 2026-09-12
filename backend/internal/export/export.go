package export

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/metrics"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type fileInfo struct {
	Src      string `json:"src"`
	Name     string `json:"name"`
	Download string `json:"download"`
	Rel      string `json:"rel"`
	AdminID  uint   `json:"admin_id,omitempty"`
	TenantID uint   `json:"tenant_id,omitempty"`
}

func Maybe(c *gin.Context, fileName string, rows any) bool {
	spec := Lookup(ctxutil.Get(c).Controller, ctxutil.Get(c).Action)
	// PHP initExport: request()->get('file_name') ?: setFileName()
	// ?: treats "" / "0" as empty; whitespace is kept.
	if name := httpx.QueryRaw(c, "file_name"); !util.PHPEmpty(name) {
		fileName = name
	} else if spec.FileName != "" {
		fileName = spec.FileName
	}
	exp := httpx.QueryInt(c, "export")
	// PHP BaseDataLists::initExport rejects lists that do not implement ListsExcelInterface.
	if (exp == 1 || exp == 2) && spec.FileName == "" && len(spec.Fields) == 0 {
		response.Fail(c, "该列表不支持导出")
		return true
	}
	if exp == 1 {
		n := rowCount(rows)
		if v, ok := c.Get("likeadmin.export_count"); ok {
			if cnt, ok := v.(int64); ok && cnt > 0 {
				n = int(cnt)
			}
		}
		pageSize := exportPageSize(c)
		if pageSize <= 0 {
			pageSize = 25
		}
		max := config.C.Project.Lists.PageSizeMax
		if max <= 0 {
			max = 10000
		}
		sum := n / pageSize
		if n%pageSize != 0 {
			sum++
		}
		if sum < 1 {
			sum = 1
		}
		response.Data(c, gin.H{
			"count": n, "page_size": pageSize, "sum_page": sum,
			"max_page": max / pageSize, "all_max_size": max,
			"page_start": 1, "page_end": min(sum, config.C.Project.Lists.ExportPages()), "file_name": fileName,
		})
		return true
	}
	if exp != 2 {
		return false
	}
	count := int64(rowCount(rows))
	if v, ok := c.Get("likeadmin.export_count"); ok {
		if cnt, ok := v.(int64); ok {
			count = cnt
		}
	}
	if msg := exportRangeError(c, count); msg != "" {
		response.Fail(c, msg)
		return true
	}
	if msg := exportWindowLimitError(c); msg != "" {
		response.Fail(c, msg)
		return true
	}
	domain := ctxutil.Domain(c)
	if v, ok := c.Get("likeadmin.export_domain"); ok {
		if s, _ := v.(string); s != "" {
			domain = s
		}
	}
	app := ctxutil.Get(c).App
	owner := exportOwner(c)
	startExportJanitor()
	if id, _ := c.Get("likeadmin.export_task_id"); id != nil {
		taskID, _ := id.(string)
		if c.GetBool("likeadmin.export_worker") {
			task, ok := loadTask(taskID)
			if !ok || task.Status != statusPending ||
				(c.Request != nil && c.Request.Context().Err() != nil) {
				return true
			}
		}
		key, err := saveOwnedXLSX(fileName, rows, spec.Fields, owner)
		if err != nil {
			if finishTask(Task{ID: taskID, Status: statusFailed, Msg: err.Error(), AdminID: owner.AdminID, TenantID: owner.TenantID}) {
				metrics.AddExport(statusFailed)
			}
			if !c.GetBool("likeadmin.export_worker") {
				response.Fail(c, err.Error())
			}
			return true
		}
		t := Task{ID: taskID, Status: statusReady, URL: downloadURL(app, domain, key, owner), File: key, AdminID: owner.AdminID, TenantID: owner.TenantID}
		if !finishTask(t) {
			discardExport(key)
			return true
		}
		metrics.AddExport(statusReady)
		if !c.GetBool("likeadmin.export_worker") {
			response.Data(c, taskPayload(t))
		}
		return true
	}
	// C-end exports retain the authenticated user context and complete in the
	// request. The background job format only carries platform/tenant admins.
	if !config.ExportAsyncEnabled() || app == "api" {
		key, err := saveOwnedXLSX(fileName, rows, spec.Fields, owner)
		if err != nil {
			response.Fail(c, err.Error())
			return true
		}
		task := newReadyTask(app, domain, key, owner)
		response.Data(c, taskPayload(task))
		return true
	}
	id := newTaskID()
	saveTask(Task{ID: id, Status: statusPending, AdminID: owner.AdminID, TenantID: owner.TenantID})
	metrics.AddExport("pending")
	fields := spec.Fields
	name := fileName
	go runExportTask(id, app, domain, name, rows, fields, owner)
	response.Data(c, gin.H{"task_id": id, "status": statusPending})
	return true
}

func SaveCSV(fileName string, rows any, fields []Field) (string, error) {
	return saveExport(fileName, rows, fields, false, TaskOwner{})
}

func SaveXLSX(fileName string, rows any, fields []Field) (string, error) {
	return saveExport(fileName, rows, fields, true, TaskOwner{})
}

func saveOwnedXLSX(fileName string, rows any, fields []Field, owner TaskOwner) (string, error) {
	return saveExport(fileName, rows, fields, true, owner)
}

func saveExport(fileName string, rows any, fields []Field, xlsx bool, owner TaskOwner) (string, error) {
	base := strings.TrimSuffix(strings.TrimSuffix(fileName, ".csv"), ".xlsx")
	if base == "" {
		base = "export"
	}
	ext := ".csv"
	if xlsx {
		ext = ".xlsx"
	}
	download := base + "-" + time.Now().Format("2006-01-02-150405") + ext
	dir := exportRoot()
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return "", err
	}
	fname := randomHex(16) + ext
	path := filepath.Join(dir, fname)
	records := toRecords(rows, fields)
	if xlsx {
		records = applyExcelLongNumbers(records)
		if err := writeXLSX(path, records); err != nil {
			return "", err
		}
	} else {
		f, err := os.Create(path)
		if err != nil {
			return "", err
		}
		w := csv.NewWriter(f)
		for _, rec := range records {
			_ = w.Write(rec)
		}
		w.Flush()
		_ = f.Close()
	}
	key := randomHex(16)
	if st, err := os.Stat(path); err == nil && st.Size() > maxFileBytes {
		_ = os.Remove(path)
		return "", fmt.Errorf("导出文件超过大小限制")
	}
	cache.Set("export_file_"+key, fileInfo{
		Name: fname, Download: download, Rel: fname,
		AdminID: owner.AdminID, TenantID: owner.TenantID,
	}, 30*time.Minute)
	return key, nil
}

func Serve(c *gin.Context) {
	startExportJanitor()
	if taskID := httpx.QueryRaw(c, "task"); taskID != "" {
		serveTask(c, taskID)
		return
	}
	key := httpx.QueryRaw(c, "file")
	var info fileInfo
	if !cache.GetJSON("export_file_"+key, &info) || info.Name == "" {
		response.Fail(c, "下载文件不存在")
		return
	}
	if !fileDownloadAllowed(c, key, info) {
		response.Fail(c, "下载文件不存在")
		return
	}
	attach := info.Download
	if attach == "" {
		attach = info.Name
	}
	dir := info.Src
	if dir == "" {
		dir = exportRoot()
	}
	path := filepath.Join(dir, info.Name)
	abs, err := filepath.Abs(path)
	if err != nil || !allowedExportPath(abs) {
		response.Fail(c, "下载文件不存在")
		return
	}
	f, err := os.Open(abs)
	if err != nil {
		response.Fail(c, "下载文件不存在")
		return
	}
	stat, err := f.Stat()
	if err != nil {
		_ = f.Close()
		response.Fail(c, "下载文件不存在")
		return
	}
	cache.Del("export_file_" + key)
	c.Header("Content-Disposition", `attachment; filename="`+attach+`"`)
	http.ServeContent(c.Writer, c.Request, attach, stat.ModTime(), f)
	_ = f.Close()
	_ = os.Remove(abs)
}

func toRecords(rows any, fields []Field) [][]string {
	if rows == nil {
		return [][]string{}
	}
	if maps := sliceOfMaps(rows); maps != nil {
		return mapsToRecords(maps, fields)
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return [][]string{{fmt.Sprint(rows)}}
	}
	var arr []map[string]any
	if json.Unmarshal(b, &arr) == nil && len(arr) > 0 {
		return mapsToRecords(arr, fields)
	}
	var raw []any
	if json.Unmarshal(b, &raw) == nil {
		out := [][]string{}
		for _, item := range raw {
			out = append(out, []string{util.ToString(item)})
		}
		return out
	}
	var buf bytes.Buffer
	buf.Write(b)
	return [][]string{{buf.String()}}
}

func sliceOfMaps(rows any) []map[string]any {
	v := reflect.ValueOf(rows)
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return nil
	}
	out := make([]map[string]any, 0, v.Len())
	for i := 0; i < v.Len(); i++ {
		item := v.Index(i)
		if item.Kind() == reflect.Interface || item.Kind() == reflect.Pointer {
			if item.IsNil() {
				continue
			}
			item = item.Elem()
		}
		switch item.Kind() {
		case reflect.Map:
			m := map[string]any{}
			for _, k := range item.MapKeys() {
				m[util.ToString(k.Interface())] = item.MapIndex(k).Interface()
			}
			out = append(out, m)
		default:
			return nil
		}
	}
	return out
}

func mapsToRecords(arr []map[string]any, fields []Field) [][]string {
	if len(fields) > 0 {
		headers := make([]string, len(fields))
		for i, f := range fields {
			headers[i] = f.Title
		}
		out := [][]string{headers}
		for _, m := range arr {
			rec := make([]string, len(fields))
			for i, f := range fields {
				rec[i] = formatCell(f.Key, m[f.Key])
			}
			out = append(out, rec)
		}
		return out
	}
	keys := make([]string, 0)
	seen := map[string]bool{}
	for _, m := range arr {
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	out := [][]string{keys}
	for _, m := range arr {
		rec := make([]string, len(keys))
		for i, k := range keys {
			rec[i] = util.ToString(m[k])
		}
		out = append(out, rec)
	}
	return out
}

func exportWindowLimitError(c *gin.Context) string {
	if exportPageType(c) != 1 {
		return ""
	}
	pages := exportPageEnd(c) - exportPageStart(c) + 1
	size := exportPageSize(c)
	maxPages := config.C.Project.Lists.ExportPages()
	if pages > maxPages {
		return fmt.Sprintf("导出范围超过限制，最多%d页", maxPages)
	}
	maxRows := config.C.Project.Lists.ExportRows()
	if pages > 0 && size > 0 && pages*size > maxRows {
		return fmt.Sprintf("导出范围超过限制，最多%d条", maxRows)
	}
	return ""
}

func rowCount(rows any) int {
	if rows == nil {
		return 0
	}
	v := reflect.ValueOf(rows)
	if v.Kind() == reflect.Slice {
		return v.Len()
	}
	return 1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func exportRoot() string {
	if d := strings.TrimSpace(os.Getenv("LIKEADMIN_EXPORT_DIR")); d != "" {
		return d
	}
	if config.C.App.PublicDir != "" {
		return filepath.Join(filepath.Dir(config.C.App.PublicDir), "runtime", "export")
	}
	return filepath.Join(os.TempDir(), "likeadmin-export")
}

func allowedExportPath(abs string) bool {
	roots := []string{exportRoot(), filepath.Join(os.TempDir(), "likeadmin-export")}
	for _, root := range roots {
		r, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if abs == r || strings.HasPrefix(abs, r+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return util.MD5(fmt.Sprintf("export-%d", time.Now().UnixNano()))
	}
	return hex.EncodeToString(b)
}

func exportOwner(c *gin.Context) TaskOwner {
	meta := ctxutil.Get(c)
	return TaskOwner{AdminID: meta.AdminID, TenantID: meta.TenantID}
}

func fileDownloadAllowed(c *gin.Context, key string, info fileInfo) bool {
	if validExportSignature(c, key, info) {
		return true
	}
	return fileOwnerOK(c, info)
}

func fileOwnerOK(c *gin.Context, info fileInfo) bool {
	if info.AdminID == 0 && info.TenantID == 0 {
		return false
	}
	return taskOwnerOK(c, Task{AdminID: info.AdminID, TenantID: info.TenantID})
}

func validExportSignature(c *gin.Context, key string, info fileInfo) bool {
	exp, err := strconv.ParseInt(httpx.QueryRaw(c, "exp"), 10, 64)
	if err != nil || exp <= 0 || time.Now().Unix() > exp {
		return false
	}
	sig := httpx.QueryRaw(c, "sig")
	if sig == "" {
		return false
	}
	want := signExportFile(key, TaskOwner{AdminID: info.AdminID, TenantID: info.TenantID}, exp)
	return hmac.Equal([]byte(want), []byte(sig))
}

func signExportFile(fileKey string, owner TaskOwner, exp int64) string {
	secret := strings.TrimSpace(config.C.Project.UniqueIdentification)
	if secret == "" {
		secret = "likeadmin"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s|%d|%d|%d", fileKey, owner.AdminID, owner.TenantID, exp)
	return hex.EncodeToString(mac.Sum(nil))
}

var janitorOnce sync.Once

func startExportJanitor() {
	janitorOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			cleanOldExports(taskTTL)
			for range ticker.C {
				cleanOldExports(taskTTL)
			}
		}()
	})
}

func cleanOldExports(maxAge time.Duration) {
	if maxAge <= 0 {
		maxAge = taskTTL
	}
	dir := exportRoot()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-maxAge)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func discardExport(key string) {
	var info fileInfo
	if !cache.GetJSON("export_file_"+key, &info) {
		return
	}
	cache.Del("export_file_" + key)
	dir := info.Src
	if dir == "" {
		dir = exportRoot()
	}
	path, err := filepath.Abs(filepath.Join(dir, info.Name))
	if err == nil && allowedExportPath(path) {
		_ = os.Remove(path)
	}
}

func exportPageType(c *gin.Context) int {
	// PHP get('page_type', 1): missing → 1; present "" / "0" → not paginated.
	if !util.PHPIsset(httpx.Query(c), "page_type") {
		return 1
	}
	return httpx.QueryInt(c, "page_type")
}

func exportPageSize(c *gin.Context) int {
	if exportPageType(c) != 1 {
		max := config.C.Project.Lists.PageSizeMax
		if max <= 0 {
			return 10000
		}
		return max
	}
	pageSize := httpx.QueryInt(c, "page_size")
	if pageSize <= 0 {
		pageSize = config.C.Project.Lists.PageSize
	}
	if pageSize <= 0 {
		return 25
	}
	return pageSize
}

func exportPageStart(c *gin.Context) int {
	if !util.PHPIsset(httpx.Query(c), "page_start") {
		return 1
	}
	return httpx.QueryInt(c, "page_start")
}

func exportPageEnd(c *gin.Context) int {
	if !util.PHPIsset(httpx.Query(c), "page_end") {
		return config.C.Project.Lists.ExportPages()
	}
	return httpx.QueryInt(c, "page_end")
}

// exportRangeError matches PHP BaseDataLists::initExport empty-range throw.
func exportRangeError(c *gin.Context, count int64) string {
	pageType := exportPageType(c)
	pageStart := exportPageStart(c)
	pageEnd := exportPageEnd(c)
	pageSize := exportPageSize(c)
	if pageSize <= 0 {
		pageSize = 25
	}
	pages := int(count) / pageSize
	if int(count)%pageSize != 0 {
		pages++
	}
	if count == 0 || pages < pageStart {
		if pageType != 0 {
			return fmt.Sprintf("第%d页到第%d页没有数据，无法导出", pageStart, pageEnd)
		}
		return "没有数据,无法导出"
	}
	return ""
}

func formatCell(key string, v any) string {
	switch strings.ToLower(key) {
	case "channel":
		if n, ok := asInt(v); ok {
			if d := util.ChannelDesc(n); d != "" {
				return d
			}
		}
	case "pay_status", "pay_status_text":
		if n, ok := asInt(v); ok {
			return util.PayStatusText(n)
		}
	case "pay_way", "pay_way_text":
		if n, ok := asInt(v); ok {
			if d := util.PayWayText(n); d != "" {
				return d
			}
		}
	}
	return util.ToString(v)
}

func applyExcelLongNumbers(records [][]string) [][]string {
	for i, rec := range records {
		if i == 0 {
			continue
		}
		for j, cell := range rec {
			if isLongNumeric(cell) {
				rec[j] = cell + "\t"
			}
		}
		records[i] = rec
	}
	return records
}

func isLongNumeric(s string) bool {
	if len(s) < 12 {
		return false
	}
	dot := 0
	for i, r := range s {
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '.' && dot == 0 {
			dot++
			continue
		}
		if r == '-' && i == 0 {
			continue
		}
		return false
	}
	return true
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int8:
		return int(n), true
	case int16:
		return int(n), true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case uint:
		return int(n), true
	case uint32:
		return int(n), true
	case uint64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		for i, r := range s {
			if r < '0' || r > '9' {
				if i != 0 || r != '-' {
					return 0, false
				}
			}
		}
		return util.ToInt(s), true
	default:
		return 0, false
	}
}
