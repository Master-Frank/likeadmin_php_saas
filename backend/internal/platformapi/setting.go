package platformapi

import (
	"context"
	"os"
	"runtime"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
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
	cfgsvc.Set(c, "platform", "name", httpx.Str(c, "name"))
	cfgsvc.Set(c, "platform", "web_favicon", filesvc.SetFileURL(c, httpx.Str(c, "web_favicon")))
	cfgsvc.Set(c, "platform", "web_logo_light", filesvc.SetFileURL(c, httpx.Str(c, "web_logo_light")))
	cfgsvc.Set(c, "platform", "web_logo_dark", filesvc.SetFileURL(c, httpx.Str(c, "web_logo_dark")))
	cfgsvc.Set(c, "platform", "login_image", filesvc.SetFileURL(c, httpx.Str(c, "login_image")))
	response.Success(c, "设置成功", nil)
}

func WebGetCopyright(c *gin.Context) {
	response.Data(c, cfgsvc.Get(c, "copyright", "config", []any{}))
}

func WebSetCopyright(c *gin.Context) {
	cfgsvc.Set(c, "copyright", "config", httpx.Any(c, "config"))
	response.Success(c, "设置成功", nil)
}

func WebGetAgreement(c *gin.Context) {
	response.Data(c, gin.H{
		"service_title":   cfgsvc.GetString(c, "agreement", "service_title", "服务协议"),
		"service_content": cfgsvc.GetString(c, "agreement", "service_content", ""),
		"privacy_title":   cfgsvc.GetString(c, "agreement", "privacy_title", "隐私政策"),
		"privacy_content": cfgsvc.GetString(c, "agreement", "privacy_content", ""),
	})
}

func WebSetAgreement(c *gin.Context) {
	cfgsvc.Set(c, "agreement", "service_title", httpx.Str(c, "service_title"))
	cfgsvc.Set(c, "agreement", "service_content", httpx.Str(c, "service_content"))
	cfgsvc.Set(c, "agreement", "privacy_title", httpx.Str(c, "privacy_title"))
	cfgsvc.Set(c, "agreement", "privacy_content", httpx.Str(c, "privacy_content"))
	response.Success(c, "设置成功", nil)
}

func UserGetConfig(c *gin.Context) {
	response.Data(c, gin.H{"default_avatar": filesvc.GetFileURL(c, cfgsvc.GetString(c, "default_image", "user_avatar", "resource/image/common/default_avatar.png"))})
}

func UserSetConfig(c *gin.Context) {
	cfgsvc.Set(c, "default_image", "user_avatar", filesvc.SetFileURL(c, httpx.Str(c, "default_avatar")))
	response.Success(c, "设置成功", nil)
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
	for _, k := range []string{"login_way", "coerce_mobile", "login_agreement", "third_auth", "wechat_auth", "qq_auth"} {
		if v, ok := httpx.Params(c)[k]; ok {
			cfgsvc.Set(c, "login", k, v)
		}
	}
	response.Success(c, "设置成功", nil)
}

func TransactionGet(c *gin.Context) {
	response.Data(c, gin.H{
		"cancel_unpaid_orders":       cfgsvc.GetInt(c, "transaction", "cancel_unpaid_orders", 0),
		"cancel_unpaid_orders_times": cfgsvc.GetInt(c, "transaction", "cancel_unpaid_orders_times", 30),
	})
}

func TransactionSet(c *gin.Context) {
	cfgsvc.Set(c, "transaction", "cancel_unpaid_orders", httpx.Int(c, "cancel_unpaid_orders"))
	cfgsvc.Set(c, "transaction", "cancel_unpaid_orders_times", httpx.Int(c, "cancel_unpaid_orders_times"))
	response.Success(c, "设置成功", nil)
}

func CustomerGet(c *gin.Context) {
	response.Data(c, gin.H{
		"way":               cfgsvc.GetInt(c, "customer_service", "way", 1),
		"phone":             cfgsvc.GetString(c, "customer_service", "phone", ""),
		"service_qr":        filesvc.GetFileURL(c, cfgsvc.GetString(c, "customer_service", "qr_code", "")),
		"wechat_qr":         filesvc.GetFileURL(c, cfgsvc.GetString(c, "customer_service", "wechat_qr", "")),
		"enterprise_wechat": cfgsvc.GetString(c, "customer_service", "enterprise_wechat", ""),
	})
}

func CustomerSet(c *gin.Context) {
	for k, v := range httpx.Params(c) {
		cfgsvc.Set(c, "customer_service", k, v)
	}
	response.Success(c, "设置成功", nil)
}

func CacheClear(c *gin.Context) {
	if bootstrap.RDB != nil {
		_ = bootstrap.RDB.FlushDB(context.Background()).Err()
	}
	response.Success(c, "清除成功", nil)
}

func SystemInfo(c *gin.Context) {
	hostname, _ := os.Hostname()
	response.Data(c, gin.H{
		"server": gin.H{"name": hostname, "os": runtime.GOOS, "arch": runtime.GOARCH, "go": runtime.Version()},
		"system": gin.H{"name": "likeadmin-saas-go", "version": "1.0.5"},
	})
}

func LogLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.OperationLog{})
	if name := lists.Param(q, "admin_name"); name != "" {
		db = db.Where("admin_name LIKE ?", "%"+name+"%")
	}
	if url := lists.Param(q, "url"); url != "" {
		db = db.Where("url LIKE ?", "%"+url+"%")
	}
	if ip := lists.Param(q, "ip"); ip != "" {
		db = db.Where("ip LIKE ?", "%"+ip+"%")
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
		desc := "正常"
		if r.Status != 1 {
			desc = "停用"
		}
		out = append(out, map[string]any{
			"id": r.ID, "name": r.Name, "type": r.Type, "status": r.Status, "remark": r.Remark,
			"create_time": util.FormatDateTime(r.CreateTime), "status_desc": desc,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func DictTypeAdd(c *gin.Context) {
	bootstrap.DB.Create(&model.DictType{Name: httpx.Str(c, "name"), Type: httpx.Str(c, "type"), Status: httpx.Int(c, "status"), Remark: httpx.Str(c, "remark"), CreateTime: util.NowUnix()})
	response.Success(c, "添加成功", nil)
}

func DictTypeEdit(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictType{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "type": httpx.Str(c, "type"), "status": httpx.Int(c, "status"), "remark": httpx.Str(c, "remark"), "update_time": now,
	})
	response.Success(c, "修改成功", nil)
}

func DictTypeDelete(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictType{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func DictTypeDetail(c *gin.Context) {
	var r model.DictType
	bootstrap.DB.First(&r, httpx.Uint(c, "id"))
	response.Data(c, r)
}

func DictTypeAll(c *gin.Context) {
	var rows []model.DictType
	bootstrap.DB.Where("delete_time IS NULL AND status = 1").Find(&rows)
	response.Data(c, rows)
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
		desc := "正常"
		if r.Status != 1 {
			desc = "停用"
		}
		item := map[string]any{
			"id": r.ID, "name": r.Name, "value": r.Value, "type_id": r.TypeID, "type_value": r.TypeValue,
			"sort": r.Sort, "status": r.Status, "remark": r.Remark, "create_time": util.FormatDateTime(r.CreateTime), "status_desc": desc,
		}
		out = append(out, item)
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func DictDataAdd(c *gin.Context) {
	bootstrap.DB.Create(&model.DictData{
		Name: httpx.Str(c, "name"), Value: httpx.Str(c, "value"), TypeID: httpx.Uint(c, "type_id"),
		TypeValue: httpx.Str(c, "type_value"), Sort: httpx.Int(c, "sort"), Status: httpx.Int(c, "status"),
		Remark: httpx.Str(c, "remark"), CreateTime: util.NowUnix(),
	})
	response.Success(c, "添加成功", nil)
}

func DictDataEdit(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictData{}).Where("id = ?", httpx.Uint(c, "id")).Updates(map[string]any{
		"name": httpx.Str(c, "name"), "value": httpx.Str(c, "value"), "type_id": httpx.Uint(c, "type_id"),
		"type_value": httpx.Str(c, "type_value"), "sort": httpx.Int(c, "sort"), "status": httpx.Int(c, "status"),
		"remark": httpx.Str(c, "remark"), "update_time": now,
	})
	response.Success(c, "修改成功", nil)
}

func DictDataDelete(c *gin.Context) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.DictData{}).Where("id = ?", httpx.Uint(c, "id")).Update("delete_time", now)
	response.Success(c, "删除成功", nil)
}

func DictDataDetail(c *gin.Context) {
	var r model.DictData
	bootstrap.DB.First(&r, httpx.Uint(c, "id"))
	response.Data(c, r)
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
	engine := httpx.Str(c, "engine")
	response.Success(c, "", cfgsvc.Get(c, "storage", engine, map[string]any{}))
}

func StorageSetup(c *gin.Context) {
	engine := httpx.Str(c, "engine")
	cfgsvc.Set(c, "storage", engine, httpx.Params(c))
	cache.Del("STORAGE_ENGINE")
	response.Success(c, "设置成功", nil)
}

func StorageChange(c *gin.Context) {
	cfgsvc.Set(c, "storage", "default", httpx.Str(c, "engine"))
	cache.Del("STORAGE_DEFAULT")
	response.Success(c, "切换成功", nil)
}
