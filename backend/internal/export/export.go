package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type fileInfo struct {
	Src      string `json:"src"`
	Name     string `json:"name"`
	Download string `json:"download"`
}

func Maybe(c *gin.Context, fileName string, rows any) bool {
	spec := Lookup(ctxutil.Get(c).Controller, ctxutil.Get(c).Action)
	// PHP initExport: request()->get('file_name') ?: setFileName()
	if name := httpx.QueryStr(c, "file_name"); name != "" {
		fileName = name
	} else if spec.FileName != "" {
		fileName = spec.FileName
	}
	exp := httpx.QueryInt(c, "export")
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
			"page_start": 1, "page_end": min(sum, 200), "file_name": fileName,
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
	key, err := SaveXLSX(fileName, rows, spec.Fields)
	if err != nil {
		response.Fail(c, err.Error())
		return true
	}
	// PHP ListsExcelTrait::createExcel always url('platformapi/download/export').
	// Download is notNeedLogin; tenant lists must keep the same prefix.
	u := ctxutil.Domain(c) + "/platformapi/download/export?file=" + key
	response.Result(c, response.CodeOpenNewPage, 1, "", gin.H{"url": u})
	return true
}

func SaveCSV(fileName string, rows any, fields []Field) (string, error) {
	return saveExport(fileName, rows, fields, false)
}

func SaveXLSX(fileName string, rows any, fields []Field) (string, error) {
	return saveExport(fileName, rows, fields, true)
}

func saveExport(fileName string, rows any, fields []Field, xlsx bool) (string, error) {
	base := strings.TrimSuffix(strings.TrimSuffix(fileName, ".csv"), ".xlsx")
	if base == "" {
		base = "export"
	}
	ext := ".csv"
	if xlsx {
		ext = ".xlsx"
	}
	download := base + "-" + time.Now().Format("2006-01-02-150405") + ext
	dir := filepath.Join(os.TempDir(), "likeadmin-export")
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(download)))
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
	key := util.MD5(path + fmt.Sprintf("%d", time.Now().UnixNano()))
	cache.Set("export_file_"+key, fileInfo{
		Src: filepath.Dir(path) + string(os.PathSeparator), Name: filepath.Base(path), Download: download,
	}, 30*time.Minute)
	return key, nil
}

func Serve(c *gin.Context) {
	key := httpx.QueryStr(c, "file")
	var info fileInfo
	if !cache.GetJSON("export_file_"+key, &info) || info.Name == "" {
		response.Fail(c, "下载文件不存在")
		return
	}
	cache.Del("export_file_" + key)
	attach := info.Download
	if attach == "" {
		attach = info.Name
	}
	c.FileAttachment(filepath.Join(info.Src, info.Name), attach)
}

func toRecords(rows any, fields []Field) [][]string {
	if rows == nil {
		return [][]string{}
	}
	b, err := json.Marshal(rows)
	if err != nil {
		return [][]string{{fmt.Sprint(rows)}}
	}
	var arr []map[string]any
	if json.Unmarshal(b, &arr) == nil && len(arr) > 0 {
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

func exportPageType(c *gin.Context) int {
	if httpx.QueryStr(c, "page_type") == "" {
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
	n := httpx.QueryInt(c, "page_start")
	if n == 0 && httpx.QueryStr(c, "page_start") == "" {
		return 1
	}
	return n
}

func exportPageEnd(c *gin.Context) int {
	n := httpx.QueryInt(c, "page_end")
	if n == 0 && httpx.QueryStr(c, "page_end") == "" {
		return 200
	}
	return n
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
	case "disable":
		if n, ok := asInt(v); ok {
			if n == 1 {
				return "禁用"
			}
			return "正常"
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
