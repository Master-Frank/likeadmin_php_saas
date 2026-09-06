package platformapi

import (
	"encoding/json"
	"strings"

	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/export"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/upgrade"
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
	if msg := util.PayWaySetCheck(data); msg != "" {
		response.Fail(c, msg)
		return
	}
	for _, raw := range data {
		arr, ok := raw.([]any)
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
			bootstrap.DB.Model(&model.PayWay{}).Where("id = ?", id).Updates(map[string]any{
				"is_default": util.ToInt(m["is_default"]), "status": util.ToInt(m["status"]),
			})
		}
	}
	response.SuccessNotice(c, "操作成功")
}

func crontabTypeDesc(t int) string {
	switch t {
	case 1:
		return "定时任务"
	case 2:
		return "守护进程"
	default:
		return ""
	}
}

func crontabStatusDesc(s int) string {
	switch s {
	case 1:
		return "运行"
	case 2:
		return "停止"
	case 3:
		return "错误"
	default:
		return ""
	}
}

func crontabWriteCheck(p map[string]any, needID bool) string {
	if needID {
		if _, ok := p["id"]; !ok || util.ToInt(p["id"]) == 0 {
			return "参数缺失"
		}
	}
	if strings.TrimSpace(util.ToString(p["name"])) == "" {
		return "请输入定时任务名称"
	}
	if _, ok := p["type"]; !ok {
		return "请选择类型"
	}
	if util.ToInt(p["type"]) != 1 {
		return "类型值错误"
	}
	if strings.TrimSpace(util.ToString(p["command"])) == "" {
		return "请输入命令"
	}
	if _, ok := p["status"]; !ok {
		return "请选择状态"
	}
	st := util.ToInt(p["status"])
	if st != 1 && st != 2 && st != 3 {
		return "状态值错误"
	}
	expr := strings.TrimSpace(util.ToString(p["expression"]))
	if expr == "" {
		return "请输入运行规则"
	}
	if !biz.ValidCron(expr) {
		return "定时任务运行规则错误"
	}
	return ""
}

func CrontabLists(c *gin.Context) {
	q := lists.Parse(c)
	var rows []model.Crontab
	var count int64
	db := bootstrap.DB.Model(&model.Crontab{}).Where("delete_time IS NULL")
	db.Count(&count)
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		last := ""
		if r.LastTime != nil && *r.LastTime > 0 {
			last = util.FormatDateTime(*r.LastTime)
		}
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "type": r.Type, "type_desc": crontabTypeDesc(r.Type),
			"command": r.Command, "params": r.Params, "expression": r.Expression,
			"status": r.Status, "status_desc": crontabStatusDesc(r.Status), "error": r.Error,
			"last_time": last, "time": r.Time, "max_time": r.MaxTime,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func CrontabAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := crontabWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Create(&model.Crontab{
		Name: httpx.Str(c, "name"), Type: httpx.Int(c, "type"), Command: httpx.Str(c, "command"),
		Params: httpx.Str(c, "params"), Status: httpx.Int(c, "status"), Expression: httpx.Str(c, "expression"),
		Remark: httpx.Str(c, "remark"), LastTime: &now, CreateTime: now,
	})
	response.SuccessNotice(c, "添加成功")
}

func CrontabEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := crontabWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	var r model.Crontab
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "定时任务不存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.Crontab{}).Where("id = ?", r.ID).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "command": httpx.Str(c, "command"), "params": httpx.Str(c, "params"),
		"status": httpx.Int(c, "status"), "expression": httpx.Str(c, "expression"), "remark": httpx.Str(c, "remark"),
		"type": httpx.Int(c, "type"), "update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func CrontabDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	now := util.NowUnix()
	res := bootstrap.DB.Model(&model.Crontab{}).Where("id = ? AND delete_time IS NULL", id).Update("delete_time", now)
	if res.RowsAffected == 0 {
		response.Fail(c, "删除失败")
		return
	}
	response.SuccessNotice(c, "删除成功")
}

func CrontabOperate(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	operate := httpx.Str(c, "operate")
	if operate == "" {
		response.Fail(c, "请选择操作")
		return
	}
	var r model.Crontab
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "定时任务不存在")
		return
	}
	status := r.Status
	switch operate {
	case "start":
		status = 1
	case "stop":
		status = 2
	default:
		response.Fail(c, "请选择操作")
		return
	}
	bootstrap.DB.Model(&r).Update("status", status)
	response.SuccessNotice(c, "操作成功")
}

func CrontabDetail(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.Crontab
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Data(c, []any{})
		return
	}
	response.Data(c, gin.H{
		"id": r.ID, "name": r.Name, "type": r.Type, "type_desc": crontabTypeDesc(r.Type),
		"command": r.Command, "params": r.Params, "status": r.Status, "status_desc": crontabStatusDesc(r.Status),
		"expression": r.Expression, "remark": r.Remark,
	})
}

func CrontabExpression(c *gin.Context) {
	expr := httpx.Str(c, "expression")
	if expr == "" {
		response.Fail(c, "请输入运行规则")
		return
	}
	lists, err := biz.CronExpressionLists(expr)
	if err != nil {
		response.Fail(c, "定时任务运行规则错误")
		return
	}
	response.Data(c, lists)
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
	response.SuccessNotice(c, "设置成功")
}

func SmsConfigGet(c *gin.Context) {
	response.Data(c, []any{
		smsEngineRow(c, "ali", "阿里云短信", 1),
		smsEngineRow(c, "tencent", "腾讯云短信", 0),
	})
}

func SmsConfigSet(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.SmsConfigWriteCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	typ := util.ToString(p["type"])
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
	response.SuccessNotice(c, "操作成功")
}

func SmsConfigDetail(c *gin.Context) {
	typ := httpx.Str(c, "type")
	def := map[string]any{"type": typ, "status": 0}
	switch typ {
	case "ali":
		def = map[string]any{"type": "ali", "name": "阿里云短信", "sign": "", "app_key": "", "secret_key": "", "status": 1}
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

func UpgradeLists(c *gin.Context) {
	q := lists.Parse(c)
	payload := upgrade.GetRemoteVersion(q.PageNo, q.PageSize)
	rawLists, _ := payload["lists"].([]any)
	count := int64(util.ToInt(payload["count"]))
	if len(rawLists) == 0 {
		response.Lists(c, []any{}, count, q.PageNo, q.PageSize, nil)
		return
	}
	response.Lists(c, upgrade.FormatLists(rawLists, q.PageNo, ""), count, q.PageNo, q.PageSize, nil)
}

func upgradeAuthMsg(result map[string]any) string {
	if msg := util.ToString(result["msg"]); msg != "" {
		return msg
	}
	return "请先联系客服获取授权"
}

func UpgradeDo(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.UpgradeCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	if msg := upgrade.CheckAbleUpgrade(p["id"]); msg != "" {
		response.Fail(c, msg)
		return
	}
	host := ctxutil.Host(c)
	result := upgrade.Verify(host, p["id"], "package_link")
	if !upgrade.HasPermission(result) {
		msg := upgradeAuthMsg(result)
		upgrade.AddLog(host, p["id"], 1, false, msg)
		response.Fail(c, "更新失败:"+msg)
		return
	}
	if err := upgrade.ApplyPackage(util.ToString(result["link"]), ""); err != nil {
		upgrade.AddLog(host, p["id"], 1, false, err.Error())
		response.Fail(c, "更新失败:"+err.Error())
		return
	}
	if ver := upgrade.VersionByID(p["id"]); ver != nil {
		_ = upgrade.WriteLocalVersion(util.ToString(ver["version_no"]))
	}
	upgrade.AddLog(host, p["id"], 1, true, "")
	response.SuccessNotice(c, "更新成功")
}

func UpgradeDownloadPkg(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.UpgradeDownloadCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	if msg := upgrade.CheckVersionData(p["id"]); msg != "" {
		response.Fail(c, msg)
		return
	}
	updateType := util.ToInt(p["update_type"])
	host := ctxutil.Host(c)
	result := upgrade.Verify(host, p["id"], upgrade.PkgLinkName(updateType))
	if !upgrade.HasPermission(result) {
		msg := upgradeAuthMsg(result)
		upgrade.AddLog(host, p["id"], updateType, false, msg)
		response.Fail(c, msg)
		return
	}
	upgrade.AddLog(host, p["id"], updateType, true, "")
	response.SuccessSilent(c, "", map[string]any{"line": result["link"]})
}

func UpgradeNotImpl(c *gin.Context) {
	UpgradeDo(c)
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
