package openapi

import (
	"fmt"
	"sort"
	"strings"

	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/sms"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ArticleAddCollect(c *gin.Context) {
	uid := ctxutil.Get(c).UserID
	aid := httpx.Uint(c, "id")
	if uid == 0 || aid == 0 {
		response.Fail(c, "参数错误")
		return
	}
	var row model.ArticleCollect
	err := tdb(c).Where("user_id = ? AND article_id = ?", uid, aid).First(&row).Error
	if err != nil {
		tdb(c).Create(&model.ArticleCollect{
			UserID: uid, ArticleID: aid, Status: 1, TenantID: ctxutil.Get(c).TenantID, CreateTime: util.NowUnix(),
		})
	} else {
		tdb(c).Model(&row).Updates(map[string]any{"status": 1, "update_time": util.NowUnix()})
	}
	response.Success(c, "操作成功", nil)
}

func ArticleCancelCollect(c *gin.Context) {
	uid := ctxutil.Get(c).UserID
	aid := httpx.Uint(c, "id")
	tdb(c).Model(&model.ArticleCollect{}).
		Where("user_id = ? AND article_id = ? AND status = 1", uid, aid).
		Updates(map[string]any{"status": 0, "update_time": util.NowUnix()})
	response.Success(c, "操作成功", nil)
}

func ArticleCollect(c *gin.Context) {
	q := lists.Parse(c)
	uid := ctxutil.Get(c).UserID
	var cols []model.ArticleCollect
	tdb(c).Where("user_id = ? AND status = 1 AND delete_time IS NULL", uid).Order("id desc").Find(&cols)
	ids := make([]uint, 0, len(cols))
	for _, col := range cols {
		ids = append(ids, col.ArticleID)
	}
	var arts []model.Article
	if len(ids) > 0 {
		tdb(c).Where("id IN ? AND is_show = 1 AND delete_time IS NULL", ids).Find(&arts)
	}
	byID := map[uint]model.Article{}
	for _, a := range arts {
		byID[a.ID] = a
	}
	filtered := make([]model.ArticleCollect, 0, len(cols))
	for _, col := range cols {
		if _, ok := byID[col.ArticleID]; ok {
			filtered = append(filtered, col)
		}
	}
	count := int64(len(filtered))
	start, end := q.Offset, q.Offset+q.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}
	out := make([]map[string]any, 0, end-start)
	for _, col := range filtered[start:end] {
		a := byID[col.ArticleID]
		out = append(out, map[string]any{
			"id": col.ID, "article_id": col.ArticleID, "title": a.Title,
			"image": filesvc.GetFileURL(c, a.Image), "desc": a.Desc, "is_show": a.IsShow,
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
			"collect_time": util.FormatDateTimeMinute(col.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleDetail(c *gin.Context) {
	id := httpx.Uint(c, "id")
	collect := false
	if uid := ctxutil.Get(c).UserID; uid > 0 && id > 0 {
		var n int64
		tdb(c).Model(&model.ArticleCollect{}).Where("user_id = ? AND article_id = ? AND status = 1", uid, id).Count(&n)
		collect = n > 0
	}
	var a model.Article
	if tdb(c).Where("id = ? AND is_show = 1 AND delete_time IS NULL", id).First(&a).Error != nil {
		response.Data(c, gin.H{"collect": collect})
		return
	}
	tdb(c).Model(&a).Update("click_actual", a.ClickActual+1)
	response.Data(c, gin.H{
		"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
		"image": filesvc.GetFileURL(c, a.Image), "author": a.Author,
		"content": filesvc.RewriteContentDomains(c, a.Content),
		"click":   a.ClickActual + a.ClickVirtual + 1, "create_time": util.FormatDateTime(a.CreateTime),
		"collect": collect,
	})
}

func RechargeCreate(c *gin.Context) {
	uid := ctxutil.Get(c).UserID
	if uid == 0 {
		response.FailSilent(c, "请求参数缺token")
		return
	}
	minAmt := util.ToFloat(cfgsvc.Get(c, "recharge", "min_amount", 0))
	if msg := util.RechargeAPICheck(httpx.Params(c), cfgsvc.GetInt(c, "recharge", "status", 0), minAmt); msg != "" {
		response.Fail(c, msg)
		return
	}
	money := httpx.Float(c, "money")
	terminal := httpx.Int(c, "terminal")
	if terminal == 0 {
		if info := ctxutil.Get(c).UserInfo; info != nil {
			terminal = util.ToInt(info["terminal"])
		}
	}
	exists := func(sn string) bool {
		var n int64
		tdb(c).Model(&model.RechargeOrder{}).Where("sn = ?", sn).Count(&n)
		return n > 0
	}
	order := model.RechargeOrder{
		SN: util.GenerateSN(exists, "", 4), UserID: uid, TenantID: ctxutil.Get(c).TenantID,
		PayStatus: 0, OrderAmount: money, OrderTerminal: terminal, CreateTime: util.NowUnix(),
	}
	if err := tdb(c).Create(&order).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Data(c, gin.H{"order_id": order.ID, "from": "recharge"})
}

func RechargeConfig(c *gin.Context) {
	u := currentUser(c)
	response.Data(c, gin.H{
		"status":     cfgsvc.GetInt(c, "recharge", "status", 0),
		"min_amount": cfgsvc.Get(c, "recharge", "min_amount", 0),
		"user_money": util.MoneyString(u.UserMoney),
	})
}

func PayWay(c *gin.Context) {
	if msg := util.PayQueryCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	from := httpx.Str(c, "from")
	orderID := httpx.Uint(c, "order_id")
	if from != "recharge" {
		response.Fail(c, "待支付订单不存在")
		return
	}
	var order model.RechargeOrder
	if tdb(c).First(&order, orderID).Error != nil {
		response.Fail(c, "待支付订单不存在")
		return
	}
	terminal := httpx.Int(c, "terminal")
	if terminal == 0 {
		if info := ctxutil.Get(c).UserInfo; info != nil {
			terminal = util.ToInt(info["terminal"])
		}
	}
	tid := ctxutil.Get(c).TenantID
	var ways []model.TenantPayWay
	q := tdb(c).Where("scene = ? AND status = 1", terminal)
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Order("is_default desc").Find(&ways)
	u := currentUser(c)
	out := make([]map[string]any, 0)
	for _, w := range ways {
		var cfg model.TenantPayConfig
		if tdb(c).First(&cfg, w.PayConfigID).Error != nil {
			continue
		}
		if from == "recharge" && cfg.PayWay == 1 {
			continue
		}
		extra := ""
		switch cfg.PayWay {
		case 1:
			extra = fmt.Sprintf("可用余额:%.2f", u.UserMoney)
		case 2:
			extra = "微信快捷支付"
		case 3:
			extra = "支付宝快捷支付"
		}
		out = append(out, map[string]any{
			"id": cfg.ID, "name": cfg.Name, "pay_way": cfg.PayWay,
			"icon": filesvc.GetFileURL(c, cfg.Icon), "sort": cfg.Sort,
			"remark": cfg.Remark, "is_default": w.IsDefault, "extra": extra,
		})
	}
	sortPayWayItems(out)
	response.Data(c, gin.H{"lists": out, "order_amount": order.OrderAmount})
}

func sortPayWayItems(out []map[string]any) {
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := util.ToInt(out[i]["is_default"]), util.ToInt(out[j]["is_default"])
		if di != dj {
			return di > dj
		}
		si, sj := util.ToInt(out[i]["sort"]), util.ToInt(out[j]["sort"])
		if si != sj {
			return si > sj
		}
		return util.ToInt(out[i]["id"]) < util.ToInt(out[j]["id"])
	})
}

func PayPrepay(c *gin.Context) {
	p := httpx.Params(c)
	if _, ok := p["from"]; !ok || strings.TrimSpace(util.ToString(p["from"])) == "" {
		response.Fail(c, "参数缺失")
		return
	}
	if _, ok := p["pay_way"]; !ok {
		response.Fail(c, "支付方式参数缺失")
		return
	}
	payWay := httpx.Int(c, "pay_way")
	if payWay != 1 && payWay != 2 && payWay != 3 {
		response.Fail(c, "支付方式参数错误")
		return
	}
	if _, ok := p["order_id"]; !ok || strings.TrimSpace(util.ToString(p["order_id"])) == "" {
		response.Fail(c, "订单参数缺失")
		return
	}
	from := httpx.Str(c, "from")
	orderID := httpx.Uint(c, "order_id")
	if from != "recharge" {
		response.Fail(c, "充值订单不存在")
		return
	}
	var order model.RechargeOrder
	if tdb(c).First(&order, orderID).Error != nil {
		response.Fail(c, "充值订单不存在")
		return
	}
	if order.PayStatus == 1 {
		response.Fail(c, "订单已支付")
		return
	}
	terminal := 0
	if info := ctxutil.Get(c).UserInfo; info != nil {
		terminal = util.ToInt(info["terminal"])
	}
	paySN := order.SN
	if payWay == 2 {
		paySN = fmt.Sprintf("%s%d%s", order.SN, terminal, fmt.Sprintf("%04d", util.NowUnix()%10000))
	}
	tdb(c).Model(&order).Updates(map[string]any{"pay_way": payWay, "pay_sn": paySN})
	order.PayWay = payWay
	order.PaySN = paySN
	if order.OrderAmount == 0 {
		if err := markRechargePaid(&order, ""); err != nil {
			response.Fail(c, err.Error())
			return
		}
		response.Success(c, "", gin.H{"pay_way": 1})
		return
	}
	if payWay == 1 {
		response.Fail(c, "充值不支持余额支付")
		return
	}
	redirect := httpx.Str(c, "redirect")
	if redirect == "" {
		redirect = "/pages/payment/payment"
	}
	var (
		data any
		err  error
	)
	switch payWay {
	case 2:
		data, err = pay.WechatPrepay(c, order, paySN, terminal, from, redirect)
	case 3:
		data, err = pay.AliPrepay(c, order, from, redirect, terminal)
	default:
		response.Fail(c, "订单异常")
		return
	}
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "", data)
}

func PayStatus(c *gin.Context) {
	if msg := util.PayQueryCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	from := httpx.Str(c, "from")
	orderID := httpx.Uint(c, "order_id")
	uid := ctxutil.Get(c).UserID
	if from != "recharge" {
		response.Fail(c, "订单不存在")
		return
	}
	var order model.RechargeOrder
	if tdb(c).Where("id = ? AND user_id = ?", orderID, uid).First(&order).Error != nil {
		response.Fail(c, "订单不存在")
		return
	}
	payDesc := map[int]string{1: "余额支付", 2: "微信支付", 3: "支付宝支付"}
	statusDesc := map[int]string{0: "未支付", 1: "已支付"}
	response.Data(c, gin.H{
		"pay_status": order.PayStatus, "pay_way": order.PayWay,
		"order": gin.H{
			"order_id": order.ID, "order_sn": order.SN, "order_amount": order.OrderAmount,
			"pay_way": payDesc[order.PayWay], "pay_status": statusDesc[order.PayStatus],
			"pay_time": util.FormatDateTimePtr(order.PayTime),
		},
	})
}

func markRechargePaid(order *model.RechargeOrder, transactionID string) error {
	db := bootstrap.DB
	if order != nil {
		db = tenantdb.ForTenant(order.TenantID)
	}
	return db.Transaction(func(tx *gorm.DB) error {
		now := util.NowUnix()
		if err := tx.Model(order).Updates(map[string]any{
			"pay_status": 1, "pay_time": now, "transaction_id": transactionID, "update_time": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).Where("id = ?", order.UserID).Updates(map[string]any{
			"user_money":            gorm.Expr("user_money + ?", order.OrderAmount),
			"total_recharge_amount": gorm.Expr("total_recharge_amount + ?", order.OrderAmount),
		}).Error; err != nil {
			return err
		}
		var user model.User
		tx.First(&user, order.UserID)
		biz.AddAccountLog(tx, order.UserID, order.TenantID, biz.UMIncRecharge, biz.INC, order.OrderAmount, user.UserMoney, order.SN, "用户充值")
		return nil
	})
}

func UserChangePassword(c *gin.Context) {
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	salt := config.C.Project.UniqueIdentification
	if u.Password != "" {
		old := httpx.Str(c, "old_password")
		if old == "" {
			response.Fail(c, "请填写旧密码")
			return
		}
		if u.Password != util.CreatePassword(old, salt) {
			response.Fail(c, "原密码不正确")
			return
		}
	}
	if msg := util.UserPasswordCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	pwd := httpx.Str(c, "password")
	tdb(c).Model(&u).Update("password", util.CreatePassword(pwd, salt))
	response.SuccessNotice(c, "操作成功")
}

func UserResetPassword(c *gin.Context) {
	mobile := httpx.Str(c, "mobile")
	code := httpx.Str(c, "code")
	if msg := util.ValidChinaMobile(mobile); msg != "" {
		if mobile == "" {
			response.Fail(c, "请输入手机号")
			return
		}
		response.Fail(c, "请输入正确手机号")
		return
	}
	if code == "" {
		response.Fail(c, "请填写验证码")
		return
	}
	if msg := util.UserPasswordCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !verifySms(c, mobile, code, "ZHDLMM") {
		response.Fail(c, "验证码错误")
		return
	}
	hashed := util.CreatePassword(httpx.Str(c, "password"), config.C.Project.UniqueIdentification)
	q := tdb(c).Model(&model.User{}).Where("mobile = ? AND delete_time IS NULL", mobile)
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Update("password", hashed)
	response.SuccessNotice(c, "操作成功")
}

func UserBindMobile(c *gin.Context) {
	if !phpRequiredParam(httpx.Params(c), "code") {
		response.Fail(c, "参数缺失")
		return
	}
	u := currentUser(c)
	mobile := httpx.Str(c, "mobile")
	code := httpx.Str(c, "code")
	typ := httpx.Str(c, "type")
	scene := "BGSJHM"
	if typ == "bind" {
		scene = "BDSJHM"
	}
	if !verifySms(c, mobile, code, scene) {
		response.Fail(c, "验证码错误")
		return
	}
	q := tdb(c).Model(&model.User{}).Where("mobile = ? AND delete_time IS NULL", mobile)
	if typ == "bind" {
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
	} else {
		q = q.Where("id = ? AND mobile = ?", u.ID, mobile)
	}
	var exist model.User
	if q.First(&exist).Error == nil {
		response.Fail(c, "该手机号已被使用")
		return
	}
	tdb(c).Model(&u).Update("mobile", mobile)
	response.SuccessNotice(c, "绑定成功")
}

func UserGetMobileByMnp(c *gin.Context) { UserGetMobileByMnpReal(c) }

func SmsSendCode(c *gin.Context) { SmsSendCodeReal(c) }

func verifySms(c *gin.Context, mobile, code, scene string) bool {
	return sms.Verify(c, mobile, code, scene)
}

func PcIndex(c *gin.Context) {
	var page model.DecoratePage
	db := tdb(c).Where("type = 4")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	_ = db.First(&page)
	response.Data(c, gin.H{
		"page": decoratePageValue(page),
		"all":  limitArticles(c, "all", 5, 0, 0),
		"new":  limitArticles(c, "new", 7, 0, 0),
		"hot":  limitArticles(c, "hot", 8, 0, 0),
	})
}

func PcConfig(c *gin.Context) {
	response.Data(c, gin.H{
		"domain": filesvc.GetFileURL(c, ""),
		"login": gin.H{
			"login_way":       cfgsvc.Get(c, "login", "login_way", []any{"1", "2"}),
			"coerce_mobile":   cfgsvc.GetInt(c, "login", "coerce_mobile", 1),
			"login_agreement": cfgsvc.GetInt(c, "login", "login_agreement", 1),
			"third_auth":      cfgsvc.GetInt(c, "login", "third_auth", 1),
			"wechat_auth":     cfgsvc.GetInt(c, "login", "wechat_auth", 1),
			"qq_auth":         cfgsvc.GetInt(c, "login", "qq_auth", 0),
		},
		"website": gin.H{
			"shop_name":   cfgsvc.GetString(c, "website", "shop_name", "likeadmin"),
			"shop_logo":   filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "shop_logo", "")),
			"pc_logo":     filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_logo", "")),
			"pc_title":    cfgsvc.GetString(c, "website", "pc_title", "likeadmin"),
			"pc_ico":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_ico", "")),
			"pc_desc":     cfgsvc.GetString(c, "website", "pc_desc", ""),
			"pc_keywords": cfgsvc.GetString(c, "website", "pc_keywords", ""),
		},
		"siteStatistics": gin.H{"clarity_code": cfgsvc.GetString(c, "siteStatistics", "clarity_code", "")},
		"version":        config.C.Project.Version,
		"copyright":      cfgsvc.Get(c, "copyright", "config", []any{}),
		"admin_url":      ctxutil.Domain(c) + "/admin",
		"qrcode": gin.H{
			"oa":  filesvc.GetFileURL(c, cfgsvc.GetString(c, "oa_setting", "qr_code", "")),
			"mnp": filesvc.GetFileURL(c, cfgsvc.GetString(c, "mnp_setting", "qr_code", "")),
		},
	})
}

func PcInfoCenter(c *gin.Context) {
	var cates []model.ArticleCate
	db := tdb(c).Where("delete_time IS NULL AND is_show = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&cates)
	out := make([]map[string]any, 0, len(cates))
	for _, cate := range cates {
		out = append(out, map[string]any{
			"id": cate.ID, "name": cate.Name,
			"article": limitArticles(c, "all", 10, int(cate.ID), 0),
		})
	}
	response.Data(c, out)
}

func PcArticleDetail(c *gin.Context) {
	id := httpx.Uint(c, "id")
	source := httpx.Str(c, "source")
	if source == "" {
		source = "default"
	}
	var a model.Article
	if tdb(c).Where("id = ? AND is_show = 1 AND delete_time IS NULL", id).First(&a).Error != nil {
		response.Fail(c, "文章不存在")
		return
	}
	tdb(c).Model(&a).Update("click_actual", a.ClickActual+1)
	list := limitArticles(c, source, 0, int(a.Cid), 0)
	nowIndex := 0
	for i, item := range list {
		if util.ToInt(item["id"]) == int(id) {
			nowIndex = i
		}
	}
	var last, next any
	if nowIndex > 0 {
		last = list[nowIndex-1]
	} else {
		last = map[string]any{}
	}
	if nowIndex+1 < len(list) {
		next = list[nowIndex+1]
	} else {
		next = map[string]any{}
	}
	collect := 0
	if uid := ctxutil.Get(c).UserID; uid > 0 {
		var n int64
		tdb(c).Model(&model.ArticleCollect{}).Where("user_id = ? AND article_id = ? AND status = 1", uid, a.ID).Count(&n)
		if n > 0 {
			collect = 1
		}
	}
	var cate model.ArticleCate
	tdb(c).First(&cate, a.Cid)
	response.Data(c, gin.H{
		"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
		"image": filesvc.GetFileURL(c, a.Image), "author": a.Author, "content": filesvc.RewriteContentDomains(c, a.Content),
		"click": a.ClickActual + a.ClickVirtual + 1, "create_time": util.FormatDateTime(a.CreateTime),
		"last": last, "next": next, "new": limitArticles(c, "new", 8, int(a.Cid), int(a.ID)),
		"collect": collect, "cate_name": cate.Name,
	})
}

func limitArticles(c *gin.Context, sortType string, limit, cate, exclude int) []map[string]any {
	db := tdb(c).Model(&model.Article{}).Where("delete_time IS NULL AND is_show = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if cate > 0 {
		db = db.Where("cid = ?", cate)
	}
	if exclude > 0 {
		db = db.Where("id <> ?", exclude)
	}
	switch sortType {
	case "new":
		db = db.Order("id desc")
	case "hot":
		db = db.Order("(click_actual + click_virtual) desc, id desc")
	default:
		db = db.Order("sort desc, id desc")
	}
	if limit > 0 {
		db = db.Limit(limit)
	}
	var rows []model.Article
	db.Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
			"image": filesvc.GetFileURL(c, a.Image), "author": a.Author,
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	return out
}

func IndexIndex(c *gin.Context) {
	var page model.DecoratePage
	db := tdb(c).Where("type = 1")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	_ = db.First(&page)
	response.Data(c, gin.H{
		"page":    decoratePageValue(page),
		"article": limitArticles(c, "new", 20, 0, 0),
	})
}

// decoratePageValue matches ThinkPHP findOrEmpty(): missing page becomes [].
func decoratePageValue(page model.DecoratePage) any {
	if page.ID == 0 {
		return []any{}
	}
	return decoratePageMap(page)
}

func decoratePageMap(page model.DecoratePage) gin.H {
	return gin.H{
		"id": page.ID, "type": page.Type, "name": page.Name,
		"data": page.Data, "meta": page.Meta, "tenant_id": page.TenantID,
		"create_time": util.FormatDateTime(page.CreateTime),
		"update_time": util.FormatDateTimeOrNil(page.UpdateTime),
	}
}

func UploadImage(c *gin.Context) {
	name, rel, errMsg := filesvc.ReceiveUpload(c, "image", "uploads/images")
	if errMsg != "" {
		response.Fail(c, errMsg)
		return
	}
	row := model.TenantFile{
		Cid: filesvc.UploadCID(c), Type: 10, Name: name, URI: rel,
		Source: filesvc.SourceUser, SourceID: ctxutil.Get(c).UserID,
		TenantID: ctxutil.Get(c).TenantID, CreateTime: util.NowUnix(),
	}
	if err := tdb(c).Create(&row).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.Success(c, "上传成功", gin.H{
		"id": row.ID, "cid": row.Cid, "type": row.Type, "name": row.Name,
		"uri": filesvc.GetFileURL(c, rel), "url": rel,
	})
}

func phpRequiredParam(p map[string]any, key string) bool {
	v, ok := p[key]
	if !ok || v == nil {
		return false
	}
	return strings.TrimSpace(util.ToString(v)) != ""
}
