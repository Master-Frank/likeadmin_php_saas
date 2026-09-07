package decorate

import (
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func DefaultStyle() map[string]any {
	if m := config.C.Project.Decorate["tabbar_style"]; m != nil {
		if obj, ok := m.(map[string]any); ok && len(obj) > 0 {
			return obj
		}
	}
	return map[string]any{"default_color": "#999999", "selected_color": "#c455ff"}
}

func Style(c *gin.Context) any {
	style := cfgsvc.Get(c, "tabbar", "style", nil)
	if style == nil || style == "" {
		style = cfgsvc.Get(c, "decorate", "tabbar_style", nil)
	}
	if style == nil || style == "" {
		return DefaultStyle()
	}
	if m, ok := style.(map[string]any); ok && len(m) == 0 {
		return DefaultStyle()
	}
	return style
}

func Lists(c *gin.Context) []map[string]any {
	out := make([]map[string]any, 0)
	tid := ctxutil.Get(c).TenantID
	if tid == 0 {
		return out
	}
	var bars []model.DecorateTabbar
	db := tenantdb.Use(c).Where("tenant_id = ?", tid)
	db.Order("id asc").Find(&bars)
	out = make([]map[string]any, 0, len(bars))
	for _, b := range bars {
		item := map[string]any{
			"id": b.ID, "name": b.Name, "tenant_id": b.TenantID, "is_show": b.IsShow,
			"selected": filesvc.GetFileURL(c, b.Selected), "unselected": filesvc.GetFileURL(c, b.Unselected),
			"link":        util.DecodeJSON(b.Link),
			"create_time": util.FormatDateTime(b.CreateTime),
			"update_time": util.FormatDateTimePtr(b.UpdateTime),
		}
		out = append(out, item)
	}
	return out
}
