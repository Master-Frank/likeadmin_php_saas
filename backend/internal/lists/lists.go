package lists

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type Query struct {
	PageNo    int
	PageSize  int
	PageType  int
	PageStart int
	PageEnd   int
	Offset    int
	Export    int
	Field     string
	OrderBy   string
	StartTime string
	EndTime   string
	Params    map[string]any
}

func Parse(c *gin.Context) Query {
	q := Query{
		PageNo:   1,
		PageSize: config.C.Project.Lists.PageSize,
		Params:   map[string]any{},
	}
	if q.PageSize <= 0 {
		q.PageSize = 25
	}
	// Search filters follow PHP request()->param() (query + body).
	// Paging / sort / time windows follow request()->get() only.
	q.Params = httpx.Params(c)
	query := httpx.Query(c)
	if v, ok := query["page_no"]; ok && util.ToInt(v) > 0 {
		q.PageNo = util.ToInt(v)
	}
	if v, ok := query["page_size"]; ok && util.ToInt(v) > 0 {
		q.PageSize = util.ToInt(v)
	}
	if q.PageSize > config.C.Project.Lists.PageSizeMax && config.C.Project.Lists.PageSizeMax > 0 {
		q.PageSize = config.C.Project.Lists.PageSizeMax
	}
	// PHP BaseDataLists::initPage: default page_type=1 paginates;
	// any other value (including 0) uses page_size_max and page_no=1.
	q.PageType = 1
	if v, ok := query["page_type"]; ok {
		q.PageType = util.ToInt(v)
	}
	if v, ok := query["export"]; ok {
		q.Export = util.ToInt(v)
	}
	q.Field = util.ToString(query["field"])
	q.OrderBy = util.ToString(query["order_by"])
	q.StartTime = util.ToString(query["start_time"])
	q.EndTime = util.ToString(query["end_time"])
	if q.PageType != 1 {
		q.PageNo = 1
		if config.C.Project.Lists.PageSizeMax > 0 {
			q.PageSize = config.C.Project.Lists.PageSizeMax
		}
	}
	q.Offset = (q.PageNo - 1) * q.PageSize
	if q.Offset < 0 {
		q.Offset = 0
	}
	// PHP ListsExcelTrait defaults; applied only for export=2 + page_type=1.
	q.PageStart = 1
	q.PageEnd = 200
	// PHP get('page_start', default): missing keeps default; present "" / "0" become 0.
	if v, ok := query["page_start"]; ok {
		q.PageStart = util.ToInt(v)
	}
	if v, ok := query["page_end"]; ok {
		q.PageEnd = util.ToInt(v)
	}
	if q.Export == 2 && q.PageType == 1 {
		perPage := q.PageSize
		q.Offset = (q.PageStart - 1) * perPage
		q.PageSize = (q.PageEnd - q.PageStart + 1) * perPage
		if q.Offset < 0 {
			q.Offset = 0
		}
	}
	return q
}

// ParseGET matches PHP ListsValidate()->get(): POST/PUT lists are rejected.
func ParseGET(c *gin.Context) (Query, bool) {
	if c != nil && c.Request != nil {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead:
		default:
			response.Fail(c, "请求方式错误，请使用get请求方式")
			return Query{}, false
		}
	}
	if msg := ValidateQuery(httpx.Query(c)); msg != "" {
		response.Fail(c, msg)
		return Query{}, false
	}
	return Parse(c), true
}

// ValidateQuery mirrors PHP ListsValidate (optional fields, GET only).
func ValidateQuery(query map[string]any) string {
	if s, ok := queryNonEmpty(query, "page_no"); ok {
		n, err := strconv.Atoi(s)
		if err != nil {
			return "page_no必须是整数"
		}
		if n <= 0 {
			return "page_no必须大于 0"
		}
	}
	if s, ok := queryNonEmpty(query, "page_size"); ok {
		n, err := strconv.Atoi(s)
		if err != nil {
			return "page_size必须是整数"
		}
		if n <= 0 {
			return "page_size必须大于 0"
		}
		max := config.C.Project.Lists.PageSizeMax
		if max > 0 && n > max {
			return fmt.Sprintf("已超出系统限制数量，请分页查询或导出，当前最多记录数为：%d", max)
		}
	}
	if s, ok := queryNonEmpty(query, "page_start"); ok {
		n, err := strconv.Atoi(s)
		if err != nil {
			return "page_start必须是整数"
		}
		if n <= 0 {
			return "page_start必须大于 0"
		}
	}
	if s, ok := queryNonEmpty(query, "page_end"); ok {
		n, err := strconv.Atoi(s)
		if err != nil {
			return "page_end必须是整数"
		}
		if n <= 0 {
			return "page_end必须大于 0"
		}
		start := 0
		if ps, ok := queryNonEmpty(query, "page_start"); ok {
			start, _ = strconv.Atoi(ps)
		}
		if n < start {
			return "导出范围设置不正确，请重新选择"
		}
	}
	if s, ok := queryNonEmpty(query, "page_type"); ok {
		if s != "0" && s != "1" {
			return "page_type必须在 0,1 范围内"
		}
	}
	if s, ok := queryNonEmpty(query, "order_by"); ok {
		dir := strings.ToLower(s)
		if dir != "desc" && dir != "asc" {
			return "order_by必须在 desc,asc 范围内"
		}
	}
	var startTime time.Time
	var hasStart bool
	if s, ok := queryNonEmpty(query, "start_time"); ok {
		t, ok := parseListDate(s)
		if !ok {
			return "start_time不是一个有效的日期"
		}
		startTime, hasStart = t, true
	}
	if s, ok := queryNonEmpty(query, "end_time"); ok {
		endTime, ok := parseListDate(s)
		if !ok {
			return "end_time不是一个有效的日期"
		}
		if hasStart && !endTime.After(startTime) {
			return "搜索的时间范围不正确"
		}
	}
	if s, ok := queryNonEmpty(query, "start"); ok && !digitsOnly(s) {
		return "start必须是数字"
	}
	if s, ok := queryNonEmpty(query, "end"); ok && !digitsOnly(s) {
		return "end必须是数字"
	}
	if s, ok := queryNonEmpty(query, "export"); ok {
		if s != "1" && s != "2" {
			return "export必须在 1,2 范围内"
		}
	}
	return ""
}

func queryNonEmpty(query map[string]any, key string) (string, bool) {
	if query == nil {
		return "", false
	}
	v, ok := query[key]
	if !ok || v == nil {
		return "", false
	}
	s := strings.TrimSpace(util.ToString(v))
	if s == "" {
		return "", false
	}
	return s, true
}

func parseListDate(s string) (time.Time, bool) {
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
		"2006/01/02 15:04:05",
		"2006/01/02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil && ts > 0 {
		return time.Unix(ts, 0), true
	}
	return time.Time{}, false
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func Param(q Query, key string) string {
	return util.ToString(q.Params[key])
}

func ParamInt(q Query, key string) int {
	return util.ToInt(q.Params[key])
}

// HasParam matches PHP ListsSearchTrait '=' filters: skip only when missing or ”.
func HasParam(q Query, key string) bool {
	v, ok := q.Params[key]
	if !ok || v == nil {
		return false
	}
	return strings.TrimSpace(util.ToString(v)) != ""
}

// PHPTruthy matches PHP model searchers that gate with `if ($value)`.
// Missing, null, false, 0, "0", and "" are skipped; whitespace is kept.
func PHPTruthy(q Query, key string) bool {
	v, ok := q.Params[key]
	if !ok || v == nil {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case []any:
		return len(t) > 0
	}
	s := util.ToString(v)
	return s != "" && s != "0"
}

// Ident returns a SQL identifier or empty if the name is unsafe.
func Ident(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return ""
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return ""
		}
	}
	return name
}

// OrderSQL builds `field asc|desc` from request params. fallback is used
// when field is empty or not an identifier. allowed, when non-empty, is an
// extra allowlist (column names).
func OrderSQL(q Query, fallback string, allowed map[string]bool) string {
	field := Ident(q.Field)
	if field == "" {
		return fallback
	}
	if len(allowed) > 0 && !allowed[field] && !allowed[strings.ToLower(field)] {
		return fallback
	}
	dir := strings.ToLower(strings.TrimSpace(q.OrderBy))
	if dir != "asc" && dir != "desc" {
		return fallback
	}
	return field + " " + dir
}
