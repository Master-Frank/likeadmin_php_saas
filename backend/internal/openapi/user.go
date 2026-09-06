package openapi

import (
	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func IndexConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"web_name":    cfgsvc.GetString(c, "website", "name", "likeadmin"),
		"web_favicon": filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "h5_favicon", "")),
		"shop_name":   cfgsvc.GetString(c, "website", "shop_name", "likeadmin"),
		"shop_logo":   filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "shop_logo", "")),
	})
}

func IndexIndex(c *gin.Context) {
	response.Data(c, gin.H{})
}

func IndexPolicy(c *gin.Context) {
	typ := httpx.Str(c, "type")
	if typ == "service" {
		response.Data(c, gin.H{"title": cfgsvc.GetString(c, "agreement", "service_title", "服务协议"), "content": cfgsvc.GetString(c, "agreement", "service_content", "")})
		return
	}
	response.Data(c, gin.H{"title": cfgsvc.GetString(c, "agreement", "privacy_title", "隐私政策"), "content": cfgsvc.GetString(c, "agreement", "privacy_content", "")})
}

func IndexDecorate(c *gin.Context) {
	var p model.DecoratePage
	db := bootstrap.DB.Where("type = ?", httpx.Int(c, "type"))
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if db.First(&p).Error != nil {
		response.Data(c, gin.H{})
		return
	}
	response.Data(c, p)
}

func LoginRegister(c *gin.Context) {
	account := httpx.Str(c, "account")
	password := httpx.Str(c, "password")
	confirm := httpx.Str(c, "password_confirm")
	if account == "" || password == "" {
		response.Fail(c, "请输入账号密码")
		return
	}
	if password != confirm {
		response.Fail(c, "两次密码不一致")
		return
	}
	tid := ctxutil.Get(c).TenantID
	var exist model.User
	if bootstrap.DB.Where("account = ? AND tenant_id = ? AND delete_time IS NULL", account, tid).First(&exist).Error == nil {
		response.Fail(c, "账号已存在")
		return
	}
	now := util.NowUnix()
	u := model.User{
		Account: account, Nickname: account,
		Password: util.CreatePassword(password, config.C.Project.UniqueIdentification),
		Channel: httpx.Int(c, "channel"), TenantID: tid, IsNewUser: 1, CreateTime: now,
		Avatar: config.C.Project.DefaultImage["user_avatar"],
	}
	// generate sn
	var maxSN int
	bootstrap.DB.Model(&model.User{}).Select("COALESCE(MAX(sn),0)").Scan(&maxSN)
	u.SN = maxSN + 1
	if err := bootstrap.DB.Create(&u).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "注册成功", nil)
}

func LoginAccount(c *gin.Context) {
	account := httpx.Str(c, "account")
	password := httpx.Str(c, "password")
	scene := httpx.Int(c, "scene")
	terminal := httpx.Int(c, "terminal")
	if terminal == 0 {
		terminal = 3
	}
	tid := ctxutil.Get(c).TenantID
	var u model.User
	q := bootstrap.DB.Where("delete_time IS NULL AND (account = ? OR mobile = ?)", account, account)
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&u).Error != nil {
		response.Fail(c, "账号不存在")
		return
	}
	if u.IsDisable == 1 {
		response.Fail(c, "账号已禁用")
		return
	}
	if scene != 2 {
		if u.Password != util.CreatePassword(password, config.C.Project.UniqueIdentification) {
			response.Fail(c, "密码错误")
			return
		}
	}
	now := util.NowUnix()
	bootstrap.DB.Model(&u).Updates(map[string]any{"login_time": now, "login_ip": ctxutil.ClientIP(c)})
	info := authsvc.SetUserToken(c, u.ID, terminal)
	response.Data(c, gin.H{
		"nickname": u.Nickname, "sn": u.SN, "mobile": u.Mobile,
		"avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"token": info["token"],
	})
}

func LoginLogout(c *gin.Context) {
	meta := ctxutil.Get(c)
	if meta.UserInfo != nil {
		authsvc.ExpireUserToken(util.ToString(meta.UserInfo["token"]))
	}
	response.Success(c, "success", nil)
}

func UserCenter(c *gin.Context) {
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	hasPwd := 0
	if u.Password != "" {
		hasPwd = 1
	}
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "sex": u.Sex, "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"mobile": u.Mobile, "create_time": util.FormatDateTime(u.CreateTime),
		"is_new_user": u.IsNewUser, "user_money": u.UserMoney, "has_password": hasPwd,
	})
}

func UserInfo(c *gin.Context) {
	u := currentUser(c)
	response.Data(c, gin.H{
		"id": u.ID, "sn": u.SN, "sex": u.Sex, "account": u.Account, "nickname": u.Nickname,
		"real_name": u.RealName, "avatar": filesvc.GetFileURL(c, firstNonEmpty(u.Avatar, config.C.Project.DefaultImage["user_avatar"])),
		"mobile": u.Mobile, "has_auth": 0, "version": config.C.Project.Version,
	})
}

func UserSetInfo(c *gin.Context) {
	u := currentUser(c)
	field := httpx.Str(c, "field")
	value := httpx.Any(c, "value")
	allow := map[string]bool{"nickname": true, "account": true, "sex": true, "avatar": true, "real_name": true}
	if !allow[field] {
		response.Fail(c, "不允许修改")
		return
	}
	if field == "avatar" {
		value = filesvc.SetFileURL(c, util.ToString(value))
	}
	bootstrap.DB.Model(&u).Update(field, value)
	response.Success(c, "修改成功", nil)
}

func ArticleLists(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.Article{}).Where("delete_time IS NULL AND is_show = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if cid := lists.ParamInt(q, "cid"); cid > 0 {
		db = db.Where("cid = ?", cid)
	}
	var count int64
	db.Count(&count)
	var rows []model.Article
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
			"image": filesvc.GetFileURL(c, a.Image), "author": a.Author,
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleCate(c *gin.Context) {
	var rows []model.ArticleCate
	db := bootstrap.DB.Where("delete_time IS NULL AND is_show = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc").Find(&rows)
	response.Data(c, rows)
}

func ArticleDetail(c *gin.Context) {
	var a model.Article
	if bootstrap.DB.First(&a, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "文章不存在")
		return
	}
	bootstrap.DB.Model(&a).Update("click_actual", a.ClickActual+1)
	response.Data(c, gin.H{
		"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
		"image": filesvc.GetFileURL(c, a.Image), "author": a.Author, "content": a.Content,
		"click": a.ClickActual + a.ClickVirtual + 1, "create_time": util.FormatDateTime(a.CreateTime),
	})
}

func SearchHot(c *gin.Context) {
	var rows []model.HotSearch
	db := bootstrap.DB
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc").Find(&rows)
	response.Data(c, rows)
}

func PcConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"pc_title":    cfgsvc.GetString(c, "website", "pc_title", "likeadmin"),
		"pc_logo":     filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_logo", "")),
		"pc_ico":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_ico", "")),
		"pc_desc":     cfgsvc.GetString(c, "website", "pc_desc", ""),
		"pc_keywords": cfgsvc.GetString(c, "website", "pc_keywords", ""),
	})
}

func PcIndex(c *gin.Context) { response.Data(c, gin.H{}) }

func RechargeConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"status":     cfgsvc.GetInt(c, "recharge", "status", 0),
		"min_amount": cfgsvc.Get(c, "recharge", "min_amount", 0),
	})
}

func RechargeLists(c *gin.Context) {
	q := lists.Parse(c)
	uid := ctxutil.Get(c).UserID
	db := bootstrap.DB.Model(&model.RechargeOrder{}).Where("user_id = ? AND delete_time IS NULL", uid)
	var count int64
	db.Count(&count)
	var rows []model.RechargeOrder
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func AccountLogLists(c *gin.Context) {
	q := lists.Parse(c)
	uid := ctxutil.Get(c).UserID
	db := bootstrap.DB.Model(&model.UserAccountLog{}).Where("user_id = ? AND delete_time IS NULL", uid)
	var count int64
	db.Count(&count)
	var rows []model.UserAccountLog
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	response.Lists(c, rows, count, q.PageNo, q.PageSize, nil)
}

func SmsSendCode(c *gin.Context) {
	response.Success(c, "发送成功", nil)
}

func WechatJsConfig(c *gin.Context) {
	response.Data(c, gin.H{"appId": "", "timestamp": util.NowUnix(), "nonceStr": "", "signature": ""})
}

func PayNotifyOK(c *gin.Context) { c.String(200, "success") }
func AliNotify(c *gin.Context)   { c.String(200, "success") }

func LoginStub(c *gin.Context) {
	response.Fail(c, "请先完成微信开放平台配置")
}

func currentUser(c *gin.Context) model.User {
	var u model.User
	id := ctxutil.Get(c).UserID
	if id == 0 {
		return u
	}
	bootstrap.DB.First(&u, id)
	return u
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
