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
	result := map[string]any{}
	for _, t := range types {
		list := []model.DictData{}
		for _, d := range rows {
			if d.TypeValue == t {
				list = append(list, d)
			}
		}
		result[t] = list
	}
	response.Data(c, result)
}

func WorkbenchIndex(c *gin.Context) {
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Unix()
	var todayNew, totalNew int64
	bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL AND create_time >= ?", todayStart).Count(&todayNew)
	bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL").Count(&totalNew)
	dates := make([]string, 0, 15)
	visitors := make([]int, 0, 15)
	sales := make([]int, 0, 15)
	for i := 14; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dates = append(dates, d.Format("01/02"))
		visitors = append(visitors, 0)
		sales = append(sales, 0)
	}
	menu := []gin.H{
		{"name": "管理员", "image": filesvc.GetFileURL(c, "resource/image/common/menu_admin.png"), "url": "/permission/admin"},
		{"name": "角色", "image": filesvc.GetFileURL(c, "resource/image/common/menu_role.png"), "url": "/permission/role"},
		{"name": "部门", "image": filesvc.GetFileURL(c, "resource/image/common/menu_dept.png"), "url": "/organization/department"},
		{"name": "字典", "image": filesvc.GetFileURL(c, "resource/image/common/menu_dict.png"), "url": "/setting/dict"},
		{"name": "代码生成器", "image": filesvc.GetFileURL(c, "resource/image/common/menu_generator.png"), "url": "/dev_tools/code"},
		{"name": "菜单权限", "image": filesvc.GetFileURL(c, "resource/image/common/menu_auth.png"), "url": "/permission/menu"},
		{"name": "网站信息", "image": filesvc.GetFileURL(c, "resource/image/common/menu_web.png"), "url": "/setting/website/information"},
		{"name": "素材中心", "image": filesvc.GetFileURL(c, "resource/image/common/menu_file.png"), "url": "/material"},
	}
	response.Data(c, gin.H{
		"version": gin.H{
			"version": config.C.Project.Version,
			"website": "www.likeadmin.cn",
			"name":    config.C.Project.ProjectName,
			"based":   "vue3.x、ElementUI、MySQL",
			"channel": gin.H{"website": "https://www.likeadmin.cn", "gitee": "https://gitee.com/likeadmin/likeadmin_php"},
		},
		"today": gin.H{
			"time":           now.Format("2006-01-02 15:04:05"),
			"today_sales":    0,
			"total_sales":    0,
			"today_visitor":  0,
			"total_visitor":  0,
			"today_new_user": todayNew,
			"total_new_user": totalNew,
			"order_num":      0,
			"order_sum":      0,
		},
		"menu":    menu,
		"visitor": gin.H{"date": dates, "list": []gin.H{{"name": "访客数", "data": visitors}}},
		"sale":    gin.H{"date": dates, "list": []gin.H{{"name": "销售量", "data": sales}}},
		"support": []gin.H{
			{"image": "", "title": "官方公众号", "desc": "关注官方公众号"},
			{"image": "", "title": "添加企业客服微信", "desc": "想了解更多请添加客服"},
		},
	})
}

func rowTime(ts int64) string { return util.FormatDateTime(ts) }
