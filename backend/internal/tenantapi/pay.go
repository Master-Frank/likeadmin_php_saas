package tenantapi

import (
	"encoding/json"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func PayConfigLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.TenantPayConfig{})
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var rows []model.TenantPayConfig
	db.Order("sort desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	names := map[int]string{1: "余额支付", 2: "微信支付", 3: "支付宝支付"}
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "pay_way": r.PayWay, "icon": filesvc.GetFileURL(c, r.Icon),
			"sort": r.Sort, "pay_way_name": names[r.PayWay],
		})
	}
	response.Lists(c, out, int64(len(rows)), q.PageNo, q.PageSize, nil)
}

func PayConfigGet(c *gin.Context) {
	var r model.TenantPayConfig
	if bootstrap.DB.First(&r, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "配置不存在")
		return
	}
	var cfg any
	_ = json.Unmarshal([]byte(r.Config), &cfg)
	response.Success(c, "", gin.H{
		"id": r.ID, "name": r.Name, "pay_way": r.PayWay, "icon": filesvc.GetFileURL(c, r.Icon),
		"sort": r.Sort, "remark": r.Remark, "config": cfg,
	})
}

func PayConfigSet(c *gin.Context) {
	id := httpx.Uint(c, "id")
	cfg, _ := json.Marshal(httpx.Any(c, "config"))
	bootstrap.DB.Model(&model.TenantPayConfig{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "icon": filesvc.SetFileURL(c, httpx.Str(c, "icon")),
		"sort": httpx.Int(c, "sort"), "config": string(cfg),
	})
	response.Success(c, "设置成功", nil)
}

func PayWayGet(c *gin.Context) {
	var rows []model.TenantPayWay
	db := bootstrap.DB
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Find(&rows)
	if len(rows) == 0 {
		response.Success(c, "", []any{})
		return
	}
	maxScene := 0
	for _, r := range rows {
		if r.Scene > maxScene {
			maxScene = r.Scene
		}
	}
	lists := map[int][]map[string]any{}
	for i := 1; i <= maxScene; i++ {
		lists[i] = []map[string]any{}
	}
	for _, r := range rows {
		var cfg model.TenantPayConfig
		bootstrap.DB.First(&cfg, r.PayConfigID)
		lists[r.Scene] = append(lists[r.Scene], map[string]any{
			"id": r.ID, "pay_config_id": r.PayConfigID, "scene": r.Scene,
			"is_default": r.IsDefault, "status": r.Status,
			"icon": filesvc.GetFileURL(c, cfg.Icon), "name": cfg.Name, "pay_way": cfg.PayWay,
		})
	}
	response.Success(c, "", lists)
}

func PayWaySet(c *gin.Context) {
	params := httpx.Params(c)
	for _, v := range params {
		arr, ok := v.([]any)
		if !ok {
			continue
		}
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			id := util.ToInt(m["id"])
			if id == 0 {
				continue
			}
			bootstrap.DB.Model(&model.TenantPayWay{}).Where("id = ?", id).Updates(map[string]any{
				"is_default": util.ToInt(m["is_default"]), "status": util.ToInt(m["status"]),
			})
		}
	}
	response.Success(c, "设置成功", nil)
}
