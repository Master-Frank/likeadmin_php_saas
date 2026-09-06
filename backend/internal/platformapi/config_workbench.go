package platformapi

import (
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/workbench"

	"github.com/gin-gonic/gin"
)

func ConfigGet(c *gin.Context) {
	response.Data(c, gin.H{
		"oss_domain":       filesvc.GetFileURL(c, ""),
		"web_name":         cfgsvc.GetString(c, "platform", "name", config.C.Project.Platform["name"]),
		"web_favicon":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "web_favicon", config.C.Project.Platform["web_favicon"])),
		"web_logo_light":   filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "web_logo_light", config.C.Project.Platform["web_logo_light"])),
		"web_logo_dark":    filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "web_logo_dark", config.C.Project.Platform["web_logo_dark"])),
		"login_image":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "login_image", config.C.Project.Platform["login_image"])),
		"copyright_config": cfgsvc.Get(c, "copyright", "config", []any{}),
	})
}

func ConfigDict(c *gin.Context) {
	typ := httpx.Str(c, "type")
	if typ == "" {
		response.Data(c, []any{})
		return
	}
	types := strings.Split(typ, ",")
	var rows []model.DictData
	bootstrap.DB.Where("type_value IN ? AND delete_time IS NULL", types).Find(&rows)
	if len(rows) == 0 {
		response.Data(c, []any{})
		return
	}
	result := map[string]any{}
	for _, t := range types {
		list := make([]map[string]any, 0)
		for _, d := range rows {
			if d.TypeValue == t {
				list = append(list, map[string]any{
					"id": d.ID, "name": d.Name, "value": d.Value, "type_id": d.TypeID,
					"type_value": d.TypeValue, "sort": d.Sort, "status": d.Status, "remark": d.Remark,
					"create_time": util.FormatDateTime(d.CreateTime),
					"update_time": util.FormatDateTimeOrNil(d.UpdateTime),
				})
			}
		}
		if len(list) > 0 {
			result[t] = list
		}
	}
	response.Data(c, result)
}

func WorkbenchIndex(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	var todayNew, totalNew int64
	bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL AND create_time >= ?", todayStart).Count(&todayNew)
	bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL").Count(&totalNew)
	vDates, vNums := workbench.Series(now, 15, 0, 100)
	sDates, sNums := workbench.Series(now, 7, 30, 200)
	response.Data(c, gin.H{
		"version": workbench.Version(c, "platform"),
		"today":   workbench.Today(now, todayNew, totalNew),
		"menu":    workbench.PlatformMenu(c),
		"visitor": gin.H{"date": vDates, "list": []gin.H{{"name": "访客数", "data": vNums}}},
		"sale":    gin.H{"date": sDates, "list": []gin.H{{"name": "销售量", "data": sNums}}},
		"support": workbench.Support(c),
	})
}

func rowTime(ts int64) string { return util.FormatDateTime(ts) }
