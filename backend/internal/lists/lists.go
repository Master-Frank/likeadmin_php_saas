package lists

import (
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
	if q.PageType == 0 && q.Export == 0 {
		// PHP: page_type 0 = no page? Looking at BaseDataLists:
		// page_type 0-一般分页；1-不分页
		// Wait, comments said: 0-一般分页；1-不分页，获取最大所有数据
	}
	if q.PageType == 1 {
		q.PageSize = config.C.Project.Lists.PageSizeMax
		q.PageNo = 1
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
