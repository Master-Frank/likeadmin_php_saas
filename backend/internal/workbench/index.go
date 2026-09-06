package workbench

import (
	"time"

	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/filesvc"

	"github.com/gin-gonic/gin"
)

func Version(c *gin.Context, typ string) gin.H {
	name := config.C.Project.ProjectName
	if typ == "platform" {
		name = cfgsvc.GetString(c, "platform", "name", config.C.Project.Platform["name"])
	} else {
		name = cfgsvc.GetString(c, "tenant", "name", config.C.Project.Tenant["name"])
	}
	website := "www.likeadmin.cn/"
	if config.C.Project.Website != nil && config.C.Project.Website["url"] != "" {
		website = config.C.Project.Website["url"]
	}
	return gin.H{
		"version": config.C.Project.Version,
		"website": website,
		"name":    name,
		"based":   "vue3.x、ElementUI、MySQL",
		"channel": gin.H{
			"website": "https://www.likeadmin.cn",
			"gitee":   "https://gitee.com/likeadmin/likeadmin_php_saas",
		},
	}
}

func Today(now time.Time, todayNew, totalNew int64) gin.H {
	return gin.H{
		"time":           now.Format("2006-01-02 15:04:05"),
		"today_sales":    100,
		"total_sales":    1000,
		"today_visitor":  10,
		"total_visitor":  100,
		"today_new_user": todayNew,
		"total_new_user": totalNew,
		"order_num":      12,
		"order_sum":      255,
	}
}

func Series(now time.Time, days int, minN, maxN int) (dates []string, nums []int) {
	dates = make([]string, 0, days)
	nums = make([]int, 0, days)
	span := maxN - minN
	if span <= 0 {
		span = 1
	}
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -i)
		dates = append(dates, d.Format("01/02"))
		nums = append(nums, minN+(int(d.Unix())%span))
	}
	return
}

func Support(c *gin.Context) []gin.H {
	return []gin.H{
		{"image": filesvc.GetFileURL(c, config.C.Project.DefaultImage["qq_group"]), "title": "官方公众号", "desc": "关注官方公众号"},
		{"image": filesvc.GetFileURL(c, config.C.Project.DefaultImage["customer_service"]), "title": "添加企业客服微信", "desc": "想了解更多请添加客服"},
	}
}

func PlatformMenu(c *gin.Context) []gin.H {
	img := func(key string) string { return filesvc.GetFileURL(c, config.C.Project.DefaultImage[key]) }
	return []gin.H{
		{"name": "管理员", "image": img("menu_admin"), "url": "/permission/admin"},
		{"name": "角色管理", "image": img("menu_role"), "url": "/permission/role"},
		{"name": "部门管理", "image": img("menu_dept"), "url": "/organization/department"},
		{"name": "字典管理", "image": img("menu_dict"), "url": "/setting/dev_tools/dict"},
		{"name": "代码生成器", "image": img("menu_generator"), "url": "/setting/dev_tools/code"},
		{"name": "素材中心", "image": img("menu_file"), "url": "/app/material/index"},
		{"name": "菜单权限", "image": img("menu_auth"), "url": "/permission/menu"},
		{"name": "网站信息", "image": img("menu_web"), "url": "/setting/website/information"},
	}
}

func TenantMenu(c *gin.Context) []gin.H {
	img := func(key string) string { return filesvc.GetFileURL(c, config.C.Project.DefaultImage[key]) }
	return []gin.H{
		{"name": "管理员", "image": img("menu_admin"), "url": "/permission/admin"},
		{"name": "角色管理", "image": img("menu_role"), "url": "/permission/role"},
		{"name": "部门管理", "image": img("menu_dept"), "url": "/organization/department"},
		{"name": "素材中心", "image": img("menu_file"), "url": "/app/material/index"},
		{"name": "菜单权限", "image": img("menu_auth"), "url": "/permission/menu"},
		{"name": "网站信息", "image": img("menu_web"), "url": "/setting/website/information"},
	}
}
