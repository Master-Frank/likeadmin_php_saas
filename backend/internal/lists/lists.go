package lists

import (
	"strings"
	"unicode"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type Query struct {
	PageNo    int
	PageSize  int
	PageType  int
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
	params := httpx.Params(c)
	q.Params = params
	if v, ok := params["page_no"]; ok && util.ToInt(v) > 0 {
		q.PageNo = util.ToInt(v)
	}
	if v, ok := params["page_size"]; ok && util.ToInt(v) > 0 {
		q.PageSize = util.ToInt(v)
	}
	if q.PageSize > config.C.Project.Lists.PageSizeMax && config.C.Project.Lists.PageSizeMax > 0 {
		q.PageSize = config.C.Project.Lists.PageSizeMax
	}
	// PHP BaseDataLists::initPage: default page_type=1 paginates;
	// any other value (including 0) uses page_size_max and page_no=1.
	q.PageType = 1
	if v, ok := params["page_type"]; ok {
		q.PageType = util.ToInt(v)
	}
	if v, ok := params["export"]; ok {
		q.Export = util.ToInt(v)
	}
	q.Field = util.ToString(params["field"])
	q.OrderBy = util.ToString(params["order_by"])
	q.StartTime = util.ToString(params["start_time"])
	q.EndTime = util.ToString(params["end_time"])
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
	return q
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
