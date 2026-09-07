package platformapi

import (
	"os"
	"path/filepath"
	"runtime"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func WebGetWebsite(c *gin.Context) {
	response.Data(c, gin.H{
		"name":           cfgsvc.GetString(c, "platform", "name", ""),
		"web_favicon":    filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "web_favicon", "")),
		"web_logo_light": filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "web_logo_light", "")),
		"web_logo_dark":  filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "web_logo_dark", "")),
		"login_image":    filesvc.GetFileURL(c, cfgsvc.GetString(c, "platform", "login_image", "")),
	})
}

func WebSetWebsite(c *gin.Context) {
	if msg := util.PlatformWebSettingCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "platform", "name", httpx.Str(c, "name"))
	cfgsvc.Set(c, "platform", "web_favicon", filesvc.SetFileURL(c, httpx.Str(c, "web_favicon")))
	cfgsvc.Set(c, "platform", "web_logo_light", filesvc.SetFileURL(c, httpx.Str(c, "web_logo_light")))
	cfgsvc.Set(c, "platform", "web_logo_dark", filesvc.SetFileURL(c, httpx.Str(c, "web_logo_dark")))
	cfgsvc.Set(c, "platform", "login_image", filesvc.SetFileURL(c, httpx.Str(c, "login_image")))
	response.SuccessNotice(c, "设置成功")
}

func WebGetCopyright(c *gin.Context) {
	response.Data(c, cfgsvc.Get(c, "copyright", "config", []any{}))
}

func WebSetCopyright(c *gin.Context) {
	cfg := httpx.Any(c, "config")
	if msg := util.CopyrightConfigCheck(cfg); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "copyright", "config", cfg)
	response.SuccessNotice(c, "设置成功")
}

func WebGetAgreement(c *gin.Context) {
	response.Data(c, gin.H{
		"service_title":   cfgsvc.Get(c, "agreement", "service_title", nil),
		"service_content": filesvc.RewriteContentDomains(c, cfgsvc.GetString(c, "agreement", "service_content", "")),
		"privacy_title":   cfgsvc.Get(c, "agreement", "privacy_title", nil),
		"privacy_content": filesvc.RewriteContentDomains(c, cfgsvc.GetString(c, "agreement", "privacy_content", "")),
	})
}

func WebSetAgreement(c *gin.Context) {
	cfgsvc.Set(c, "agreement", "service_title", httpx.Str(c, "service_title"))
	cfgsvc.Set(c, "agreement", "service_content", filesvc.ClearContentDomains(c, httpx.Str(c, "service_content")))
	cfgsvc.Set(c, "agreement", "privacy_title", httpx.Str(c, "privacy_title"))
	cfgsvc.Set(c, "agreement", "privacy_content", filesvc.ClearContentDomains(c, httpx.Str(c, "privacy_content")))
	response.SuccessNotice(c, "设置成功")
}

func UserGetConfig(c *gin.Context) {
	response.Data(c, gin.H{"default_avatar": filesvc.GetFileURL(c, cfgsvc.GetString(c, "default_image", "user_avatar", "resource/image/common/default_avatar.png"))})
}

func UserSetConfig(c *gin.Context) {
	if msg := util.UserAvatarCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "default_image", "user_avatar", filesvc.SetFileURL(c, httpx.Str(c, "default_avatar")))
	response.SuccessNotice(c, "操作成功")
}

func UserGetRegisterConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"login_way":       cfgsvc.Get(c, "login", "login_way", []any{"1", "2"}),
		"coerce_mobile":   cfgsvc.GetInt(c, "login", "coerce_mobile", 1),
		"login_agreement": cfgsvc.GetInt(c, "login", "login_agreement", 1),
		"third_auth":      cfgsvc.GetInt(c, "login", "third_auth", 1),
		"wechat_auth":     cfgsvc.GetInt(c, "login", "wechat_auth", 1),
		"qq_auth":         cfgsvc.GetInt(c, "login", "qq_auth", 0),
	})
}

func UserSetRegisterConfig(c *gin.Context) {
	if msg := util.UserRegisterConfigCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	for _, k := range []string{"login_way", "coerce_mobile", "login_agreement", "third_auth", "wechat_auth", "qq_auth"} {
		if v, ok := httpx.Params(c)[k]; ok {
			cfgsvc.Set(c, "login", k, v)
		}
	}
	response.SuccessNotice(c, "操作成功")
}

func TransactionGet(c *gin.Context) {
	response.Data(c, gin.H{
		"cancel_unpaid_orders":       cfgsvc.GetInt(c, "transaction", "cancel_unpaid_orders", 1),
		"cancel_unpaid_orders_times": cfgsvc.GetInt(c, "transaction", "cancel_unpaid_orders_times", 30),
		"verification_orders":        cfgsvc.GetInt(c, "transaction", "verification_orders", 1),
		"verification_orders_times":  cfgsvc.GetInt(c, "transaction", "verification_orders_times", 24),
	})
}

func TransactionSet(c *gin.Context) {
	if msg := util.TransactionSettingCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "transaction", "cancel_unpaid_orders", httpx.Int(c, "cancel_unpaid_orders"))
	cfgsvc.Set(c, "transaction", "verification_orders", httpx.Int(c, "verification_orders"))
	if _, ok := httpx.Params(c)["cancel_unpaid_orders_times"]; ok {
		cfgsvc.Set(c, "transaction", "cancel_unpaid_orders_times", httpx.Int(c, "cancel_unpaid_orders_times"))
	}
	if _, ok := httpx.Params(c)["verification_orders_times"]; ok {
		cfgsvc.Set(c, "transaction", "verification_orders_times", httpx.Int(c, "verification_orders_times"))
	}
	response.SuccessNotice(c, "操作成功")
}

func CustomerGet(c *gin.Context) {
	qr := cfgsvc.GetString(c, "customer_service", "qr_code", "")
	if qr != "" {
		qr = filesvc.GetFileURL(c, qr)
	}
	response.Data(c, gin.H{
		"qr_code":      qr,
		"wechat":       cfgsvc.GetString(c, "customer_service", "wechat", ""),
		"phone":        cfgsvc.GetString(c, "customer_service", "phone", ""),
		"service_time": cfgsvc.GetString(c, "customer_service", "service_time", ""),
	})
}

func CustomerSet(c *gin.Context) {
	p := httpx.Params(c)
	for _, k := range []string{"qr_code", "wechat", "phone", "service_time"} {
		v, ok := p[k]
		if !ok {
			continue
		}
		if k == "qr_code" {
			cfgsvc.Set(c, "customer_service", k, filesvc.SetFileURL(c, util.ToString(v)))
			continue
		}
		cfgsvc.Set(c, "customer_service", k, v)
	}
	response.SuccessNotice(c, "设置成功")
}

func CacheClear(c *gin.Context) {
	cache.Flush()
	clearRuntimeFileCache()
	response.SuccessNotice(c, "清除成功")
}

func clearRuntimeFileCache() {
	root := filepath.Join(config.C.App.PublicDir, "..", "runtime", "file")
	if config.C.App.PublicDir == "" {
		root = filepath.Join("runtime", "file")
	}
	_ = os.RemoveAll(root)
	_ = os.MkdirAll(root, 0o755)
}

func SystemInfo(c *gin.Context) {
	writable := 0
	runtimeDir := filepath.Join(config.C.App.PublicDir, "..", "runtime")
	if config.C.App.PublicDir == "" {
		runtimeDir = "runtime"
	}
	if err := os.MkdirAll(runtimeDir, 0o755); err == nil {
		probe := filepath.Join(runtimeDir, ".write_probe")
		if err := os.WriteFile(probe, []byte("ok"), 0o644); err == nil {
			writable = 1
			_ = os.Remove(probe)
		}
	}
	response.Data(c, gin.H{
		"server": []gin.H{
			{"param": "服务器操作系统", "value": runtime.GOOS},
			{"param": "web服务器环境", "value": "Go " + runtime.Version()},
			{"param": "PHP版本", "value": runtime.Version()},
		},
		"env": []gin.H{
			{"option": "PHP版本", "require": "8.0版本以上", "status": 1, "remark": "Go 后端已替代 PHP 运行时"},
		},
		"auth": []gin.H{
			{"dir": "/runtime", "require": "runtime目录可写", "status": writable, "remark": ""},
		},
	})
}

func LogLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.OperationLog{})
	if ctxutil.Get(c).App == "tenantapi" {
		db = db.Where("url LIKE ?", "%/tenantapi/%")
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			adb := tenantdb.Use(c)
			if adb == nil {
				adb = bootstrap.DB
			}
			var ids []uint
			adb.Model(&model.TenantAdmin{}).Where("tenant_id = ? AND delete_time IS NULL", tid).Pluck("id", &ids)
			if len(ids) == 0 {
				db = db.Where("1 = 0")
			} else {
				db = db.Where("admin_id IN ?", ids)
			}
		}
	}
	if name := lists.Param(q, "admin_name"); name != "" {
		db = db.Where("admin_name LIKE ?", "%"+name+"%")
	}
	if url := lists.Param(q, "url"); url != "" {
		db = db.Where("url LIKE ?", "%"+url+"%")
	}
	if ip := lists.Param(q, "ip"); ip != "" {
		db = db.Where("ip LIKE ?", "%"+ip+"%")
	}
	if typ := lists.Param(q, "type"); typ != "" {
		db = db.Where("type LIKE ?", "%"+typ+"%")
	}
	startTS, endTS := util.ParseDateTime(q.StartTime), util.ParseDateTime(q.EndTime)
	if startTS > 0 && endTS > 0 {
		db = db.Where("create_time BETWEEN ? AND ?", startTS, endTS)
	}
	var count int64
	db.Count(&count)
	var rows []model.OperationLog
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "action": r.Action, "admin_name": r.AdminName, "admin_id": r.AdminID,
			"url": r.URL, "type": r.Type, "params": r.Params, "ip": r.IP,
			"create_time": util.FormatDateTime(r.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func DictTypeLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.DictType{}).Where("delete_time IS NULL")
	if n := lists.Param(q, "name"); n != "" {
		db = db.Where("name LIKE ?", "%"+n+"%")
	}
	if t := lists.Param(q, "type"); t != "" {
		db = db.Where("type LIKE ?", "%"+t+"%")
	}
	if lists.Param(q, "status") != "" {
		db = db.Where("status = ?", lists.ParamInt(q, "status"))
	}
	var count int64
	db.Count(&count)
	var rows []model.DictType
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dictTypeMap(r))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func DictTypeAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.DictTypeWriteCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	var n int64
	bootstrap.DB.Model(&model.DictType{}).Where("type = ? AND delete_time IS NULL", httpx.Str(c, "type")).Count(&n)
	if n > 0 {
		response.Fail(c, "字典类型已存在")
		return
	}
	bootstrap.DB.Create(&model.DictType{Name: httpx.Str(c, "name"), Type: httpx.Str(c, "type"), Status: httpx.Int(c, "status"), Remark: httpx.Str(c, "remark"), CreateTime: util.NowUnix()})
	response.SuccessNotice(c, "添加成功")
}

func DictTypeEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.DictType
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "字典类型不存在")
		return
	}
	p := httpx.Params(c)
	if msg := util.DictTypeWriteCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	var n int64
	bootstrap.DB.Model(&model.DictType{}).Where("type = ? AND id <> ? AND delete_time IS NULL", httpx.Str(c, "type"), id).Count(&n)
	if n > 0 {
		response.Fail(c, "字典类型已存在")
		return
	}
	now := util.NowUnix()
	typ := httpx.Str(c, "type")
	bootstrap.DB.Model(&model.DictType{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "type": typ, "status": httpx.Int(c, "status"), "remark": httpx.Str(c, "remark"), "update_time": now,
	})
	bootstrap.DB.Model(&model.DictData{}).Where("type_id = ?", id).Update("type_value", typ)
	response.SuccessNotice(c, "编辑成功")
}

func DictTypeDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.DictType
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "字典类型不存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictType{}).Where("id = ?", id).Update("delete_time", now)
	response.SuccessNotice(c, "删除成功")
}

func DictTypeDetail(c *gin.Context) {
	id := httpx.QueryUint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.DictType
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "字典类型不存在")
		return
	}
	response.Data(c, dictTypeRaw(r))
}

func DictTypeAll(c *gin.Context) {
	var rows []model.DictType
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Order("id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dictTypeRaw(r))
	}
	response.Data(c, out)
}

func DictDataLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.DictData{}).Where("delete_time IS NULL")
	if n := lists.Param(q, "name"); n != "" {
		db = db.Where("name LIKE ?", "%"+n+"%")
	}
	if t := lists.Param(q, "type_value"); t != "" {
		db = db.Where("type_value LIKE ?", "%"+t+"%")
	}
	if lists.Param(q, "status") != "" {
		db = db.Where("status = ?", lists.ParamInt(q, "status"))
	}
	if lists.ParamInt(q, "type_id") > 0 {
		db = db.Where("type_id = ?", lists.ParamInt(q, "type_id"))
	}
	var count int64
	db.Count(&count)
	var rows []model.DictData
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, dictDataMap(r))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func DictDataAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.DictDataWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	var typ model.DictType
	if bootstrap.DB.Where("delete_time IS NULL").First(&typ, httpx.Uint(c, "type_id")).Error != nil {
		response.Fail(c, "字典类型不存在")
		return
	}
	typeVal := httpx.Str(c, "type_value")
	if typeVal == "" {
		typeVal = typ.Type
	}
	bootstrap.DB.Create(&model.DictData{
		Name: httpx.Str(c, "name"), Value: httpx.Str(c, "value"), TypeID: typ.ID,
		TypeValue: typeVal, Sort: httpx.Int(c, "sort"), Status: httpx.Int(c, "status"),
		Remark: httpx.Str(c, "remark"), CreateTime: util.NowUnix(),
	})
	response.SuccessNotice(c, "添加成功")
}

func DictDataEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.DictData
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "字典数据不存在")
		return
	}
	p := httpx.Params(c)
	if msg := util.DictDataWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictData{}).Where("id = ?", id).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "value": httpx.Str(c, "value"),
		"sort": httpx.Int(c, "sort"), "status": httpx.Int(c, "status"),
		"remark": httpx.Str(c, "remark"), "update_time": now,
	})
	response.SuccessNotice(c, "编辑成功")
}

func DictDataDelete(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.DictData
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "字典数据不存在")
		return
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictData{}).Where("id = ?", id).Update("delete_time", now)
	response.SuccessNotice(c, "删除成功")
}

func DictDataDetail(c *gin.Context) {
	id := httpx.QueryUint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var r model.DictData
	if bootstrap.DB.Where("delete_time IS NULL").First(&r, id).Error != nil {
		response.Fail(c, "字典数据不存在")
		return
	}
	response.Data(c, dictDataRaw(r))
}

func dictTypeRaw(r model.DictType) map[string]any {
	return map[string]any{
		"id": r.ID, "name": r.Name, "type": r.Type, "status": r.Status, "remark": r.Remark,
		"create_time": util.FormatDateTime(r.CreateTime),
		"update_time": util.FormatDateTimeOrNil(r.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(r.DeleteTime),
	}
}

func dictTypeMap(r model.DictType) map[string]any {
	desc := "正常"
	if r.Status != 1 {
		desc = "停用"
	}
	out := dictTypeRaw(r)
	out["status_desc"] = desc
	return out
}

func dictDataRaw(r model.DictData) map[string]any {
	return map[string]any{
		"id": r.ID, "name": r.Name, "value": r.Value, "type_id": r.TypeID, "type_value": r.TypeValue,
		"sort": r.Sort, "status": r.Status, "remark": r.Remark,
		"create_time": util.FormatDateTime(r.CreateTime),
		"update_time": util.FormatDateTimeOrNil(r.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(r.DeleteTime),
	}
}

func dictDataMap(r model.DictData) map[string]any {
	desc := "正常"
	if r.Status != 1 {
		desc = "停用"
	}
	out := dictDataRaw(r)
	out["status_desc"] = desc
	return out
}

func StorageLists(c *gin.Context) {
	def := cfgsvc.GetString(c, "storage", "default", "local")
	out := []gin.H{
		{"name": "本地存储", "path": "存储在本地服务器", "engine": "local", "status": bool01(def == "local")},
		{"name": "七牛云存储", "path": "存储在七牛云，请前往七牛云开通存储服务", "engine": "qiniu", "status": bool01(def == "qiniu")},
		{"name": "阿里云OSS", "path": "存储在阿里云，请前往阿里云开通存储服务", "engine": "aliyun", "status": bool01(def == "aliyun")},
		{"name": "腾讯云COS", "path": "存储在腾讯云，请前往腾讯云开通存储服务", "engine": "qcloud", "status": bool01(def == "qcloud")},
	}
	response.Success(c, "获取成功", out)
}

func bool01(ok bool) int {
	if ok {
		return 1
	}
	return 0
}

func StorageDetail(c *gin.Context) {
	engine := httpx.QueryStr(c, "engine")
	if engine == "" {
		response.Fail(c, "engine不能为空")
		return
	}
	def := cfgsvc.GetString(c, "storage", "default", "")
	row := map[string]any{"status": 0}
	switch engine {
	case "local":
		row = map[string]any{"status": 0}
	case "qiniu", "aliyun":
		row = asStorageMap(cfgsvc.Get(c, "storage", engine, map[string]any{
			"bucket": "", "access_key": "", "secret_key": "", "domain": "", "status": 0,
		}))
	case "qcloud":
		row = asStorageMap(cfgsvc.Get(c, "storage", engine, map[string]any{
			"bucket": "", "region": "", "access_key": "", "secret_key": "", "domain": "", "status": 0,
		}))
	default:
		response.Fail(c, "engine不能为空")
		return
	}
	if engine == def {
		row["status"] = 1
	} else {
		row["status"] = 0
	}
	response.Success(c, "获取成功", row)
}

func StorageSetup(c *gin.Context) {
	engine := httpx.Str(c, "engine")
	if engine == "" {
		response.Fail(c, "engine不能为空")
		return
	}
	if _, ok := httpx.Params(c)["status"]; !ok {
		response.Fail(c, "status不能为空")
		return
	}
	status := httpx.Int(c, "status")
	if status == 1 {
		cfgsvc.Set(c, "storage", "default", engine)
	} else {
		cfgsvc.Set(c, "storage", "default", "local")
	}
	switch engine {
	case "local":
		cfgsvc.Set(c, "storage", "local", map[string]any{})
	case "qiniu", "aliyun":
		cfgsvc.Set(c, "storage", engine, map[string]any{
			"bucket": httpx.Str(c, "bucket"), "access_key": httpx.Str(c, "access_key"),
			"secret_key": httpx.Str(c, "secret_key"), "domain": httpx.Str(c, "domain"),
		})
	case "qcloud":
		cfgsvc.Set(c, "storage", engine, map[string]any{
			"bucket": httpx.Str(c, "bucket"), "region": httpx.Str(c, "region"),
			"access_key": httpx.Str(c, "access_key"), "secret_key": httpx.Str(c, "secret_key"),
			"domain": httpx.Str(c, "domain"),
		})
	}
	cache.Del("STORAGE_DEFAULT")
	cache.Del("STORAGE_ENGINE")
	if engine == "local" && status == 0 {
		response.SuccessNotice(c, "默认开启本地存储")
		return
	}
	response.SuccessNotice(c, "配置成功")
}

func StorageChange(c *gin.Context) {
	engine := httpx.Str(c, "engine")
	if engine == "" {
		response.Fail(c, "engine不能为空")
		return
	}
	def := cfgsvc.GetString(c, "storage", "default", "local")
	if def == engine {
		cfgsvc.Set(c, "storage", "default", "local")
	} else {
		cfgsvc.Set(c, "storage", "default", engine)
	}
	cache.Del("STORAGE_DEFAULT")
	cache.Del("STORAGE_ENGINE")
	response.SuccessNotice(c, "切换成功")
}

func asStorageMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}
