package platformapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/export"
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
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "id不能为空")
		return
	}
	var r model.PayConfig
	if bootstrap.DB.First(&r, id).Error != nil {
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
	var r model.PayConfig
	exists := bootstrap.DB.First(&r, id).Error == nil && r.ID > 0
	var taken int64
	if name := httpx.Str(c, "name"); name != "" {
		bootstrap.DB.Model(&model.PayConfig{}).Where("name = ? AND id <> ?", name, id).Count(&taken)
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
	bootstrap.DB.Model(&model.PayConfig{}).Where("id = ?", id).Updates(map[string]any{
		"name": in.Name, "icon": filesvc.SetFileURL(c, in.Icon), "sort": httpx.Int(c, "sort"),
		"config": biz.BuildPayConfigJSON(r.PayWay, in.Config), "remark": in.Remark,
	})
	response.SuccessNotice(c, "设置成功")
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
			"icon": filesvc.GetFileURL(c, cfg.Icon), "pay_way_name": cfg.Name,
		})
	}
	response.Success(c, "获取成功", grouped)
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
	db.Order("id asc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		sms := util.DecodeJSON(r.SmsNotice)
		out = append(out, map[string]any{
			"id": r.ID, "scene_name": r.SceneName, "sms_notice": sms, "type": r.Type,
			"sms_status_desc": smsStatusDesc(r.SmsNotice), "type_desc": noticeTypeDesc(r.Type),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func NoticeDetail(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.NoticeSetting
	if bootstrap.DB.First(&r, id).Error != nil || r.ID == 0 {
		response.Data(c, []any{})
		return
	}
	response.Data(c, biz.FormatNoticeDetail(
		r.ID, r.Type, r.SceneID, r.SceneName, r.SceneDesc,
		r.SystemNotice, r.SmsNotice, r.OaNotice, r.MnpNotice, r.Support,
	))
}

func NoticeSet(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var r model.NoticeSetting
	exists := bootstrap.DB.First(&r, id).Error == nil && r.ID > 0
	updates, err := biz.ApplyNoticeSet(exists, id, httpx.Any(c, "template"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	bootstrap.DB.Model(&model.NoticeSetting{}).Where("id = ?", id).Updates(updates)
	response.Success(c, "设置成功", nil)
}

func SmsConfigGet(c *gin.Context) {
	response.Data(c, []any{
		smsEngineRow(c, "ali", "阿里云短信", 0),
		smsEngineRow(c, "tencent", "腾讯云短信", 0),
	})
}

func SmsConfigSet(c *gin.Context) {
	p := httpx.Params(c)
	typ := util.ToString(p["type"])
	if typ == "" {
		typ = httpx.Str(c, "type")
	}
	if typ == "" {
		typ = "ali"
	}
	p["type"] = typ
	if util.ToString(p["name"]) == "" {
		if typ == "tencent" {
			p["name"] = "腾讯云短信"
		} else {
			p["name"] = "阿里云短信"
		}
	}
	cfgsvc.Set(c, "sms", typ, p)
	if util.ToInt(p["status"]) == 1 {
		engine := strings.ToUpper(typ)
		current := strings.ToUpper(cfgsvc.GetString(c, "sms", "engine", ""))
		if current != "" && current != engine {
			oldName := strings.ToLower(current)
			if oldName == "aliyun" {
				oldName = "ali"
			}
			old := asCfgMap(cfgsvc.Get(c, "sms", oldName, map[string]any{}))
			old["status"] = 0
			cfgsvc.Set(c, "sms", oldName, old)
		}
		cfgsvc.Set(c, "sms", "engine", engine)
	}
	response.Success(c, "设置成功", nil)
}

func SmsConfigDetail(c *gin.Context) {
	typ := httpx.Str(c, "type")
	def := map[string]any{"type": typ, "status": 0}
	switch typ {
	case "ali":
		def = map[string]any{"type": "ali", "name": "阿里云短信", "sign": "", "app_key": "", "secret_key": "", "status": 0}
	case "tencent":
		def = map[string]any{"type": "tencent", "name": "腾讯云短信", "sign": "", "app_id": "", "secret_id": "", "secret_key": "", "status": 0}
	}
	row := asCfgMap(cfgsvc.Get(c, "sms", typ, def))
	row["status"] = util.ToInt(row["status"])
	response.Data(c, row)
}

func smsEngineRow(c *gin.Context, typ, name string, status int) map[string]any {
	row := asCfgMap(cfgsvc.Get(c, "sms", typ, map[string]any{"type": typ, "name": name, "status": status}))
	if util.ToString(row["type"]) == "" {
		row["type"] = typ
	}
	if util.ToString(row["name"]) == "" {
		row["name"] = name
	}
	row["status"] = util.ToInt(row["status"])
	return row
}

func asCfgMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}

const upgradeProductCode = "462953db655787cb99deb5893f8d523a"

func UpgradeLists(c *gin.Context) {
	q := lists.Parse(c)
	key := fmt.Sprintf("version_lists%d", q.PageNo)
	var payload map[string]any
	if raw, ok := cache.Get(key); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &payload)
	}
	if payload == nil {
		url := fmt.Sprintf("https://server.mddai.cn/indexapi/version/lists?type=2&page_no=%d&page_size=%d&page=1&action=lists&product_code=%s",
			q.PageNo, q.PageSize, upgradeProductCode)
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			var wrap map[string]any
			if json.Unmarshal(body, &wrap) == nil {
				if data, ok := wrap["data"].(map[string]any); ok {
					payload = data
					if b, err := json.Marshal(data); err == nil {
						cache.Set(key, string(b), 30*time.Minute)
					}
				}
			}
		}
	}
	if payload == nil {
		response.Lists(c, []any{}, 0, q.PageNo, q.PageSize, nil)
		return
	}
	rawLists, _ := payload["lists"].([]any)
	count := int64(util.ToInt(payload["count"]))
	response.Lists(c, formatUpgradeLists(rawLists), count, q.PageNo, q.PageSize, nil)
}

func formatUpgradeLists(rows []any) []map[string]any {
	local := config.C.Project.Version
	out := make([]map[string]any, 0, len(rows))
	for _, item := range rows {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		ver := util.ToString(m["version_no"])
		m["version_str"] = ""
		m["able_update"] = 0
		if local == ver {
			m["version_str"] = "您的系统当前处于此版本"
		} else if local < ver {
			m["version_str"] = "系统可更新至此版本"
			m["able_update"] = 1
		}
		m["new_version"] = 0
		notice := []any{}
		if util.ToInt(m["uniapp_publish"]) == 1 {
			notice = append(notice, "更新至当前版本后需重新发布手机端前端前台")
		}
		if util.ToInt(m["pc_admin_publish"]) == 1 {
			notice = append(notice, "更新至当前版本后需重新发布前端PC后台")
		}
		if util.ToInt(m["pc_shop_publish"]) == 1 {
			notice = append(notice, "更新至当前版本后需重新发布前端PC前台")
		}
		if extra := util.ToString(m["publish_content"]); extra != "" {
			notice = append(notice, extra)
		}
		m["notice"] = notice
		out = append(out, m)
	}
	if len(out) > 0 {
		out[0]["new_version"] = 1
	}
	return out
}

func UpgradeNotImpl(c *gin.Context) {
	response.Fail(c, "在线升级面向 PHP 发行包，Go 版请通过发版更新")
}

func DownloadExport(c *gin.Context) {
	export.Serve(c)
}

func smsStatusDesc(raw string) string {
	if raw == "" {
		return "停用"
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return "停用"
	}
	if util.ToInt(m["status"]) == 1 {
		return "启用"
	}
	return "停用"
}

func noticeTypeDesc(t int) string {
	switch t {
	case 1:
		return "业务通知"
	case 2:
		return "验证码"
	default:
		return ""
	}
}
