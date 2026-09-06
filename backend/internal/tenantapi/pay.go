package tenantapi

import (
	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/ctxutil"
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
	db := tdb(c).Model(&model.TenantPayConfig{})
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
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "id不能为空")
		return
	}
	var r model.TenantPayConfig
	if tdb(c).First(&r, id).Error != nil {
		response.Fail(c, "支付方式不存在")
		return
	}
	response.Success(c, "获取成功", biz.PayConfigView(
		r.ID, r.Name, r.PayWay, filesvc.GetFileURL(c, r.Icon), r.Sort, r.Remark, r.Config, ctxutil.Domain(c),
	))
}

func PayConfigSet(c *gin.Context) {
	p := httpx.Params(c)
	id := httpx.Uint(c, "id")
	var r model.TenantPayConfig
	exists := tdb(c).First(&r, id).Error == nil && r.ID > 0
	var taken int64
	if name := httpx.Str(c, "name"); name != "" {
		q := tdb(c).Model(&model.TenantPayConfig{}).Where("name = ? AND id <> ?", name, id)
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Count(&taken)
	}
	_, sortOK := p["sort"]
	_, cfgOK := p["config"]
	in := biz.PayConfigInput{
		ID: id, Name: httpx.Str(c, "name"), Icon: httpx.Str(c, "icon"), Remark: httpx.Str(c, "remark"),
		Sort: httpx.Any(c, "sort"), SortPresent: sortOK, Config: httpx.Any(c, "config"), ConfigPresent: cfgOK,
		PayWay: r.PayWay, Exists: exists, NameTaken: taken > 0,
	}
	if msg := biz.CheckPayConfig(in); msg != "" {
		response.Fail(c, msg)
		return
	}
	tdb(c).Model(&model.TenantPayConfig{}).Where("id = ?", id).Updates(map[string]any{
		"name": in.Name, "icon": filesvc.SetFileURL(c, in.Icon), "sort": httpx.Int(c, "sort"),
		"config": biz.BuildPayConfigJSON(r.PayWay, in.Config), "remark": in.Remark,
	})
	response.SuccessNotice(c, "设置成功")
}

func PayWayGet(c *gin.Context) {
	var rows []model.TenantPayWay
	db := tdb(c)
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
		tdb(c).First(&cfg, r.PayConfigID)
		lists[r.Scene] = append(lists[r.Scene], map[string]any{
			"id": r.ID, "pay_config_id": r.PayConfigID, "scene": r.Scene,
			"is_default": r.IsDefault, "status": r.Status,
			"icon": filesvc.GetFileURL(c, cfg.Icon), "pay_way_name": cfg.Name,
		})
	}
	response.Success(c, "获取成功", lists)
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
			tdb(c).Model(&model.TenantPayWay{}).Where("id = ?", id).Updates(map[string]any{
				"is_default": util.ToInt(m["is_default"]), "status": util.ToInt(m["status"]),
			})
		}
	}
	response.Success(c, "设置成功", nil)
}
