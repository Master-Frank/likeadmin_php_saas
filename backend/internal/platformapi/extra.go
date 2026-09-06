package platformapi

import (
	"encoding/json"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func PayConfigLists(c *gin.Context) {
	q := lists.Parse(c)
	var rows []model.PayConfig
	bootstrap.DB.Order("sort desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	names := map[int]string{1: "余额支付", 2: "微信支付", 3: "支付宝支付"}
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "pay_way": r.PayWay, "icon": r.Icon, "sort": r.Sort,
			"pay_way_name": names[r.PayWay],
		})
	}
	response.Lists(c, out, int64(len(rows)), q.PageNo, q.PageSize, nil)
}

func PayConfigGet(c *gin.Context) {
	var r model.PayConfig
	if bootstrap.DB.First(&r, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "配置不存在")
		return
	}
	var cfg any
	_ = json.Unmarshal([]byte(r.Config), &cfg)
	response.Success(c, "", gin.H{
		"id": r.ID, "name": r.Name, "pay_way": r.PayWay, "icon": r.Icon, "sort": r.Sort, "remark": r.Remark, "config": cfg,
	})
}

func PayConfigSet(c *gin.Context) {
	id := httpx.Uint(c, "id")
	cfg, _ := json.Marshal(httpx.Any(c, "config"))
	bootstrap.DB.Model(&model.PayConfig{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "icon": httpx.Str(c, "icon"), "sort": httpx.Int(c, "sort"), "config": string(cfg),
	})
	response.Success(c, "设置成功", nil)
}

func PayWayGet(c *gin.Context) {
	var rows []model.PayWay
	bootstrap.DB.Find(&rows)
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
	grouped := map[int][]map[string]any{}
	for i := 1; i <= maxScene; i++ {
		grouped[i] = []map[string]any{}
	}
	for _, r := range rows {
		var cfg model.PayConfig
		bootstrap.DB.First(&cfg, r.PayConfigID)
		grouped[r.Scene] = append(grouped[r.Scene], map[string]any{
			"id": r.ID, "pay_config_id": r.PayConfigID, "scene": r.Scene,
			"is_default": r.IsDefault, "status": r.Status,
			"icon": cfg.Icon, "name": cfg.Name, "pay_way": cfg.PayWay,
		})
	}
	response.Success(c, "", grouped)
}

func PayWaySet(c *gin.Context) {
	data := httpx.Params(c)
	if raw, ok := data["data"]; ok {
		if arr, ok := raw.([]any); ok {
			for _, item := range arr {
				m, _ := item.(map[string]any)
				id := util.ToInt(m["id"])
				bootstrap.DB.Model(&model.PayWay{}).Where("id = ?", id).Updates(map[string]any{
					"is_default": util.ToInt(m["is_default"]), "status": util.ToInt(m["status"]),
				})
			}
		}
	}
	response.Success(c, "设置成功", nil)
}

func CrontabLists(c *gin.Context) {
	q := lists.Parse(c)
	var rows []model.Crontab
	var count int64
	bootstrap.DB.Model(&model.Crontab{}).Count(&count)
	bootstrap.DB.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	typeDesc := map[int]string{1: "定时任务"}
	statusDesc := map[int]string{1: "运行", 2: "停止", 3: "错误"}
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "type": r.Type, "type_desc": typeDesc[r.Type],
			"command": r.Command, "params": r.Params, "expression": r.Expression,
			"status": r.Status, "status_desc": statusDesc[r.Status], "error": r.Error,
			"last_time": util.FormatDateTimePtr(r.LastTime), "time": r.Time, "max_time": r.MaxTime,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func CrontabAdd(c *gin.Context) {
	bootstrap.DB.Create(&model.Crontab{
		Name: httpx.Str(c, "name"), Type: httpx.Int(c, "type"), Command: httpx.Str(c, "command"),
		Params: httpx.Str(c, "params"), Status: httpx.Int(c, "status"), Expression: httpx.Str(c, "expression"), Remark: httpx.Str(c, "remark"),
	})
	response.Success(c, "添加成功", nil)
}

func CrontabEdit(c *gin.Context) {
	bootstrap.DB.Model(&model.Crontab{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "command": httpx.Str(c, "command"), "params": httpx.Str(c, "params"),
		"status": httpx.Int(c, "status"), "expression": httpx.Str(c, "expression"), "remark": httpx.Str(c, "remark"),
	})
	response.Success(c, "修改成功", nil)
}

func CrontabDelete(c *gin.Context) {
	bootstrap.DB.Delete(&model.Crontab{}, httpx.Uint(c, "id"))
	response.Success(c, "删除成功", nil)
}

func CrontabOperate(c *gin.Context) {
	bootstrap.DB.Model(&model.Crontab{}).Where("id = ?", httpx.Uint(c, "id")).Update("status", httpx.Int(c, "status"))
	response.Success(c, "操作成功", nil)
}

func CrontabDetail(c *gin.Context) {
	var r model.Crontab
	bootstrap.DB.First(&r, httpx.Uint(c, "id"))
	response.Data(c, r)
}

func CrontabExpression(c *gin.Context) {
	response.Data(c, gin.H{"lists": []string{}})
}

func NoticeSettingLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.NoticeSetting{})
	if lists.Param(q, "recipient") != "" {
		db = db.Where("recipient = ?", lists.ParamInt(q, "recipient"))
	}
	if lists.Param(q, "type") != "" {
		db = db.Where("type = ?", lists.ParamInt(q, "type"))
	}
	var count int64
	db.Count(&count)
	var rows []model.NoticeSetting
	db.Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "scene_name": r.SceneName, "sms_notice": r.SmsNotice, "type": r.Type,
			"sms_status_desc": "", "type_desc": "",
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func NoticeDetail(c *gin.Context) {
	var r model.NoticeSetting
	bootstrap.DB.First(&r, httpx.Uint(c, "id"))
	response.Data(c, r)
}

func NoticeSet(c *gin.Context) {
	id := httpx.Uint(c, "id")
	p := httpx.Params(c)
	b, _ := json.Marshal(p)
	_ = b
	updates := map[string]any{}
	if v, ok := p["sms_notice"]; ok {
		raw, _ := json.Marshal(v)
		updates["sms_notice"] = string(raw)
	}
	if v, ok := p["system_notice"]; ok {
		raw, _ := json.Marshal(v)
		updates["system_notice"] = string(raw)
	}
	if len(updates) > 0 {
		bootstrap.DB.Model(&model.NoticeSetting{}).Where("id = ?", id).Updates(updates)
	}
	response.Success(c, "设置成功", nil)
}

func SmsConfigGet(c *gin.Context) {
	response.Data(c, gin.H{
		"ali":     cfgsvc.Get(c, "sms", "ali", map[string]any{}),
		"tencent": cfgsvc.Get(c, "sms", "tencent", map[string]any{}),
	})
}

func SmsConfigSet(c *gin.Context) {
	typ := httpx.Str(c, "type")
	if typ == "" {
		typ = "ali"
	}
	cfgsvc.Set(c, "sms", typ, httpx.Params(c))
	response.Success(c, "设置成功", nil)
}

func SmsConfigDetail(c *gin.Context) {
	response.Data(c, cfgsvc.Get(c, "sms", httpx.Str(c, "type"), map[string]any{}))
}

func UpgradeNotImpl(c *gin.Context) {
	response.Fail(c, "在线升级面向 PHP 发行包，Go 版请通过发版更新")
}
