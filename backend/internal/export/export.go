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
	Src  string `json:"src"`
	Name string `json:"name"`
}

func Maybe(c *gin.Context, fileName string, rows any) bool {
	spec := Lookup(ctxutil.Get(c).Controller, ctxutil.Get(c).Action)
	if spec.FileName != "" {
		fileName = spec.FileName
	}
	exp := httpx.Int(c, "export")
	if exp == 1 {
		n := rowCount(rows)
		if v, ok := c.Get("likeadmin.export_count"); ok {
			if cnt, ok := v.(int64); ok && cnt > 0 {
				n = int(cnt)
			}
		}
		pageSize := httpx.Int(c, "page_size")
		if pageSize <= 0 {
			pageSize = config.C.Project.Lists.PageSize
		}
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
	key, err := SaveCSV(fileName, rows, spec.Fields)
	if err != nil {
		response.Fail(c, err.Error())
		return true
	}
	app := ctxutil.Get(c).App
	if app == "" {
		app = "platformapi"
	}
	u := ctxutil.Domain(c) + "/" + app + "/download/export?file=" + key
	response.Result(c, response.CodeOpenNewPage, 1, "", gin.H{"url": u})
	return true
}

func SaveCSV(fileName string, rows any, fields []Field) (string, error) {
	if fileName == "" {
		fileName = "export.csv"
	}
	if filepath.Ext(fileName) == "" {
		fileName += ".csv"
	}
	dir := filepath.Join(os.TempDir(), "likeadmin-export")
	if err := os.MkdirAll(dir, 0o775); err != nil {
		return "", err
	}
	path := filepath.Join(dir, fmt.Sprintf("%d_%s", time.Now().UnixNano(), filepath.Base(fileName)))
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	records := toRecords(rows, fields)
	for _, rec := range records {
		_ = w.Write(rec)
	}
	w.Flush()
	key := util.MD5(path + fmt.Sprintf("%d", time.Now().UnixNano()))
	cache.Set("export_file_"+key, fileInfo{Src: filepath.Dir(path) + string(os.PathSeparator), Name: filepath.Base(path)}, 30*time.Minute)
	return key, nil
}

func Serve(c *gin.Context) {
	key := c.Query("file")
	if key == "" {
		key = httpx.Str(c, "file")
	}
	var info fileInfo
	if !cache.GetJSON("export_file_"+key, &info) || info.Name == "" {
		response.Fail(c, "下载文件不存在")
		return
	}
	cache.Del("export_file_" + key)
	c.FileAttachment(filepath.Join(info.Src, info.Name), info.Name)
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
