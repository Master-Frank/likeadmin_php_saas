package openapi

import (
	"sort"

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
	"likeadmin/backend/internal/ratelimit"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/sms"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ArticleAddCollect(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	uid := ctxutil.Get(c).UserID
	if uid == 0 {
		response.Fail(c, "参数错误")
		return
	}
	// PHP ArticleController::addCollect uses post('id/d') (query is ignored).
	aid := httpx.BodyUint(c, "id")
	var row model.ArticleCollect
	err := articleCollectDB(c).Where("user_id = ? AND article_id = ?", uid, aid).First(&row).Error
	if err != nil {
		now := util.NowUnix()
		tdb(c).Create(&model.ArticleCollect{
			UserID: uid, ArticleID: aid, Status: 1, TenantID: ctxutil.Get(c).TenantID, CreateTime: now, UpdateTime: util.UnixPtr(now),
		})
	} else {
		articleCollectDB(c).Where("id = ?", row.ID).Updates(map[string]any{"status": 1, "update_time": util.NowUnix()})
	}
	response.Success(c, "操作成功", nil)
}

func ArticleCancelCollect(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	uid := ctxutil.Get(c).UserID
	aid := httpx.BodyUint(c, "id")
	articleCollectDB(c).Where("user_id = ? AND article_id = ? AND status = 1", uid, aid).
		Updates(map[string]any{"status": 0, "update_time": util.NowUnix()})
	response.Success(c, "操作成功", nil)
}

func ArticleCollect(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	uid := ctxutil.Get(c).UserID
	at := tenantdb.Table(c, model.Article{}.TableName())
	ct := tenantdb.Table(c, model.ArticleCollect{}.TableName())
	db := tdb(c).Table(at+" AS a").Joins("JOIN "+ct+" AS c ON c.article_id = a.id").
		Where("c.user_id = ? AND c.status = 1 AND a.is_show = 1 AND c.delete_time IS NULL AND a.delete_time IS NULL", uid)
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		db = db.Where("c.tenant_id = ? AND a.tenant_id = ?", tid, tid)
	} else {
		db = db.Where("1 = 0")
	}
	var count int64
	db.Count(&count)
	type row struct {
		ID           uint   `gorm:"column:id"`
		ArticleID    uint   `gorm:"column:article_id"`
		Title        string `gorm:"column:title"`
		Image        string `gorm:"column:image"`
		Desc         string `gorm:"column:desc"`
		IsShow       int    `gorm:"column:is_show"`
		ClickVirtual int    `gorm:"column:click_virtual"`
		ClickActual  int    `gorm:"column:click_actual"`
		CreateTime   int64  `gorm:"column:create_time"`
		CollectTime  int64  `gorm:"column:collect_time"`
	}
	var rows []row
	db.Select("c.id,c.article_id,a.title,a.image,a.desc,a.is_show,a.click_virtual,a.click_actual,a.create_time,c.create_time AS collect_time").
		Order("a.sort desc, c.id desc").Offset(q.Offset).Limit(q.PageSize).Scan(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "article_id": r.ArticleID, "title": r.Title,
			"image": filesvc.GetImageAttr(c, r.Image), "desc": r.Desc, "is_show": r.IsShow,
			"click": r.ClickActual + r.ClickVirtual, "create_time": util.FormatDateTime(r.CreateTime),
			"collect_time": util.FormatDateTimeMinute(r.CollectTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func ArticleDetail(c *gin.Context) {
	id := httpx.QueryUint(c, "id")
	collect := userCollectsArticle(c, ctxutil.Get(c).UserID, id)
	var a model.Article
	if scopeTenant(tdb(c).Where("id = ? AND is_show = 1 AND delete_time IS NULL", id), c).First(&a).Error != nil {
		response.Data(c, gin.H{"collect": collect})
		return
	}
	tdb(c).Model(&model.Article{}).Where("id = ?", a.ID).Updates(map[string]any{
		"click_actual": gorm.Expr("click_actual + 1"), "update_time": util.NowUnix(),
	})
	out := articleDetailMap(c, a, a.ClickActual+a.ClickVirtual+1)
	out["collect"] = collect
	response.Data(c, out)
}

func userTerminal(c *gin.Context) int {
	if info := ctxutil.Get(c).UserInfo; info != nil {
		return util.ToInt(info["terminal"])
	}
	return 0
}

func RechargeCreate(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	uid := ctxutil.Get(c).UserID
	if uid == 0 {
		response.FailSilent(c, "请求参数缺token")
		return
	}
	minAmt := util.ToFloat(cfgsvc.Get(c, "recharge", "min_amount", 0))
	if msg := util.RechargeAPICheck(httpx.Body(c), cfgsvc.GetInt(c, "recharge", "status", 0), minAmt); msg != "" {
		response.Fail(c, msg)
		return
	}
	money := httpx.BodyFloat(c, "money")
	terminal := userTerminal(c)
	tid := ctxutil.Get(c).TenantID
	if tid == 0 {
		response.Fail(c, "接口域名错误或租户不存在")
		return
	}
	now := util.NowUnix()
	exists := func(sn string) bool {
		var n int64
		tdb(c).Model(&model.RechargeOrder{}).Where("sn = ? AND tenant_id = ? AND delete_time IS NULL", sn, tid).Count(&n)
		return n > 0
	}
	order := model.RechargeOrder{
		SN: util.GenerateSN(exists, "", 4), UserID: uid, TenantID: tid,
		PayStatus: 0, OrderAmount: money, OrderTerminal: terminal, CreateTime: now, UpdateTime: util.UnixPtr(now),
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
	if msg := util.PayQueryCheck(httpx.Query(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	from := httpx.QueryRaw(c, "from")
	orderID := httpx.QueryUint(c, "order_id")
	if from != "recharge" {
		response.Fail(c, "待支付订单不存在")
		return
	}
	var order model.RechargeOrder
	// PHP PaymentLogic::getPayWay uses findOrEmpty(order_id) with no user_id filter.
	if scopeTenant(tdb(c).Where("id = ? AND delete_time IS NULL", orderID), c).First(&order).Error != nil {
		response.Fail(c, "待支付订单不存在")
		return
	}
	terminal := userTerminal(c)
	var ways []model.TenantPayWay
	scopeTenant(tdb(c).Where("scene = ? AND status = 1", terminal), c).Order("is_default desc").Find(&ways)
	u := currentUser(c)
	cfgIDs := make([]uint, 0, len(ways))
	seen := map[uint]bool{}
	for _, w := range ways {
		if w.PayConfigID == 0 || seen[w.PayConfigID] {
			continue
		}
		seen[w.PayConfigID] = true
		cfgIDs = append(cfgIDs, w.PayConfigID)
	}
	cfgByID := map[uint]model.TenantPayConfig{}
	if len(cfgIDs) > 0 {
		var cfgs []model.TenantPayConfig
		scopeTenant(tdb(c).Where("id IN ?", cfgIDs), c).Find(&cfgs)
		for _, cfg := range cfgs {
			cfgByID[cfg.ID] = cfg
		}
	}
	out := make([]map[string]any, 0)
	for _, w := range ways {
		cfg, ok := cfgByID[w.PayConfigID]
		if !ok {
			continue
		}
		if from == "recharge" && cfg.PayWay == 1 {
			continue
		}
		extra := ""
		switch cfg.PayWay {
		case 1:
			extra = "可用余额:" + util.MoneyString(u.UserMoney)
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
	response.Data(c, gin.H{"lists": out, "order_amount": util.MoneyString(order.OrderAmount)})
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
	if !response.RequirePOST(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.PayPayCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	payWay := httpx.BodyInt(c, "pay_way")
	from := httpx.BodyRaw(c, "from")
	orderID := httpx.BodyUint(c, "order_id")
	if from != "recharge" {
		response.FailWithData(c, "充值订单不存在", p)
		return
	}
	var order model.RechargeOrder
	// PHP PaymentLogic::getPayOrderInfo looks up by order_id only.
	if scopeTenant(tdb(c).Where("id = ? AND delete_time IS NULL", orderID), c).First(&order).Error != nil {
		response.FailWithData(c, "充值订单不存在", p)
		return
	}
	if order.PayStatus == 1 {
		response.FailWithData(c, "订单已支付", p)
		return
	}
	terminal := userTerminal(c)
	paySN := order.SN
	if payWay == 2 {
		paySN = pay.FormatPaySN(order.SN, terminal, util.NowUnix())
	}
	tdb(c).Model(&order).Updates(map[string]any{
		"pay_way": payWay, "pay_sn": paySN, "update_time": util.NowUnix(),
	})
	order.PayWay = payWay
	order.PaySN = paySN
	if order.OrderAmount == 0 {
		if err := markRechargePaid(&order, ""); err != nil {
			response.FailWithData(c, err.Error(), p)
			return
		}
		response.SuccessSilent(c, "", gin.H{"pay_way": 1})
		return
	}
	if payWay == 1 {
		// PHP PaymentLogic::pay switch is wechat/ali/default → 订单异常.
		response.FailWithData(c, "订单异常", p)
		return
	}
	// PHP: $params['redirect'] ?? '/pages/payment/payment' — empty string is kept.
	redirect := "/pages/payment/payment"
	if util.PHPIsset(p, "redirect") {
		redirect = httpx.BodyRaw(c, "redirect")
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
		response.FailWithData(c, "订单异常", p)
		return
	}
	if err != nil {
		response.FailWithData(c, err.Error(), p)
		return
	}
	response.SuccessSilent(c, "", data)
}

func PayStatus(c *gin.Context) {
	if msg := util.PayQueryCheck(httpx.Query(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	from := httpx.QueryRaw(c, "from")
	orderID := httpx.QueryUint(c, "order_id")
	uid := ctxutil.Get(c).UserID
	if from != "recharge" {
		response.Fail(c, "订单不存在")
		return
	}
	var order model.RechargeOrder
	if scopeTenant(tdb(c).Where("id = ? AND user_id = ? AND delete_time IS NULL", orderID, uid), c).First(&order).Error != nil {
		response.Fail(c, "订单不存在")
		return
	}
	payDesc := map[int]string{1: "余额支付", 2: "微信支付", 3: "支付宝支付"}
	statusDesc := map[int]string{0: "未支付", 1: "已支付"}
	response.Data(c, gin.H{
		"pay_status": order.PayStatus, "pay_way": order.PayWay,
		"order": gin.H{
			"order_id": order.ID, "order_sn": order.SN, "order_amount": util.MoneyString(order.OrderAmount),
			"pay_way": payDesc[order.PayWay], "pay_status": statusDesc[order.PayStatus],
			"pay_time": util.FormatDateTimePtr(order.PayTime),
		},
	})
}

func markRechargePaid(order *model.RechargeOrder, transactionID string) error {
	if order == nil || bootstrap.DB == nil {
		return nil
	}
	// RechargeOrder stays on the shared table (PHP $notCheckTables). User /
	// account log rewrite onto la_user_{sn} when tactics=1, on the same txn.
	return bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		now := util.NowUnix()
		q := tx.Model(&model.RechargeOrder{}).Where("id = ? AND pay_status = 0 AND delete_time IS NULL AND tenant_id = ?", order.ID, order.TenantID)
		res := q.Updates(map[string]any{
			"pay_status": 1, "pay_time": now, "transaction_id": transactionID, "update_time": now,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil
		}
		udb := tenantdb.ForTenantOn(tx, order.TenantID)
		uq := udb.Model(&model.User{}).Where("id = ? AND tenant_id = ? AND delete_time IS NULL", order.UserID, order.TenantID)
		if err := uq.Updates(map[string]any{
			"user_money":            gorm.Expr("user_money + ?", order.OrderAmount),
			"total_recharge_amount": gorm.Expr("total_recharge_amount + ?", order.OrderAmount),
			"update_time":           now,
		}).Error; err != nil {
			return err
		}
		var user model.User
		udb.Where("id = ? AND tenant_id = ? AND delete_time IS NULL", order.UserID, order.TenantID).First(&user)
		biz.AddAccountLog(udb, order.UserID, order.TenantID, biz.UMIncRecharge, biz.INC, order.OrderAmount, user.UserMoney, order.SN, "用户充值")
		return nil
	})
}

func UserChangePassword(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	u := currentUser(c)
	if u.ID == 0 {
		response.Fail(c, "请先登录")
		return
	}
	salt := config.C.Project.UniqueIdentification
	// PHP PasswordValidate runs before UserLogic::changePassword.
	if msg := util.UserPasswordCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	if u.Password != "" {
		// PHP UserLogic::changePassword uses empty() on the raw old_password.
		old := httpx.BodyRaw(c, "old_password")
		if util.PHPEmpty(old) {
			response.Fail(c, "请填写旧密码")
			return
		}
		if u.Password != util.CreatePassword(old, salt) {
			response.Fail(c, "原密码不正确")
			return
		}
	}
	pwd := httpx.BodyRaw(c, "password")
	tdb(c).Model(&u).Updates(map[string]any{
		"password": util.CreatePassword(pwd, salt), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "操作成功")
}

func UserResetPassword(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	mobile := httpx.BodyRaw(c, "mobile")
	if msg := util.ValidChinaMobile(mobile); msg != "" {
		if !util.PHPRequired(httpx.Body(c), "mobile") {
			response.Fail(c, "请输入手机号")
			return
		}
		response.Fail(c, "请输入正确手机号")
		return
	}
	if !util.PHPRequired(httpx.Body(c), "code") {
		response.Fail(c, "请填写验证码")
		return
	}
	code := httpx.BodyRaw(c, "code")
	if msg := util.UserPasswordCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	if !verifySms(c, mobile, code, "ZHDLMM") {
		response.Fail(c, "验证码错误")
		return
	}
	hashed := util.CreatePassword(httpx.BodyRaw(c, "password"), config.C.Project.UniqueIdentification)
	scopeTenant(tdb(c).Model(&model.User{}).Where("mobile = ? AND delete_time IS NULL", mobile), c).Updates(map[string]any{
		"password": hashed, "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "操作成功")
}

func UserBindMobile(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !util.PHPRequired(httpx.Body(c), "code") {
		response.Fail(c, "参数缺失")
		return
	}
	u := currentUser(c)
	// PHP UserValidate::sceneBindMobile is only code.require; mobile format is not checked.
	mobile := httpx.BodyRaw(c, "mobile")
	code := httpx.BodyRaw(c, "code")
	typ := httpx.BodyRaw(c, "type")
	scene := "BGSJHM"
	if typ == "bind" {
		scene = "BDSJHM"
	}
	if !verifySms(c, mobile, code, scene) {
		response.Fail(c, "验证码错误")
		return
	}
	// PHP bindMobile: type=bind checks any user with this mobile; change only
	// rejects when the current user already has it (duplicates across users allowed).
	q := scopeTenant(tdb(c).Model(&model.User{}).Where("mobile = ? AND delete_time IS NULL", mobile), c)
	if typ != "bind" {
		q = q.Where("id = ?", u.ID)
	}
	var exist model.User
	if q.First(&exist).Error == nil {
		response.Fail(c, "该手机号已被使用")
		return
	}
	tdb(c).Model(&u).Updates(map[string]any{"mobile": mobile, "update_time": util.NowUnix()})
	response.SuccessNotice(c, "绑定成功")
}

func UserGetMobileByMnp(c *gin.Context) { UserGetMobileByMnpReal(c) }

func SmsSendCode(c *gin.Context) { SmsSendCodeReal(c) }

func verifySms(c *gin.Context, mobile, code, scene string) bool {
	return sms.Verify(c, mobile, code, scene)
}

func PcIndex(c *gin.Context) {
	var page model.DecoratePage
	db := scopeTenant(tdb(c).Where("type = 4"), c)
	_ = db.First(&page)
	response.Data(c, gin.H{
		"page": decoratePageValue(page),
		"all":  limitArticles(c, "all", 5, 0, 0),
		"new":  limitArticles(c, "new", 7, 0, 0),
		"hot":  limitArticles(c, "hot", 8, 0, 0),
	})
}

func qrFileURL(c *gin.Context, uri string) any {
	return filesvc.FileURLUnlessEmpty(c, uri)
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
			"shop_name":   cfgsvc.GetString(c, "website", "shop_name", ""),
			"shop_logo":   filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "shop_logo", "")),
			"pc_logo":     filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_logo", "")),
			"pc_title":    cfgsvc.GetString(c, "website", "pc_title", ""),
			"pc_ico":      filesvc.GetFileURL(c, cfgsvc.GetString(c, "website", "pc_ico", "")),
			"pc_desc":     cfgsvc.GetString(c, "website", "pc_desc", ""),
			"pc_keywords": cfgsvc.GetString(c, "website", "pc_keywords", ""),
		},
		"siteStatistics": gin.H{"clarity_code": cfgsvc.GetString(c, "siteStatistics", "clarity_code", "")},
		"version":        config.C.Project.Version,
		"copyright":      cfgsvc.Get(c, "copyright", "config", []any{}),
		"admin_url":      ctxutil.Domain(c) + "/admin",
		"qrcode": gin.H{
			"oa":  qrFileURL(c, cfgsvc.GetString(c, "oa_setting", "qr_code", "")),
			"mnp": qrFileURL(c, cfgsvc.GetString(c, "mnp_setting", "qr_code", "")),
		},
	})
}

func PcInfoCenter(c *gin.Context) {
	var cates []model.ArticleCate
	db := scopeTenant(tdb(c).Where("delete_time IS NULL AND is_show = 1"), c)
	db.Order("sort desc, id desc").Find(&cates)
	out := make([]map[string]any, 0, len(cates))
	for _, cate := range cates {
		out = append(out, map[string]any{
			"id": cate.ID, "name": cate.Name,
			// PHP ArticleCate::article() hasMany has no is_show filter.
			"article": queryArticles(c, "all", 10, int(cate.ID), 0, false),
		})
	}
	response.Data(c, out)
}

func PcArticleDetail(c *gin.Context) {
	id := httpx.QueryUint(c, "id")
	// PHP: get('source/s', 'default') — missing uses default; "" / "  " stay.
	source := "default"
	if util.PHPIsset(httpx.Query(c), "source") {
		source = httpx.QueryRaw(c, "source")
	}
	var a model.Article
	if scopeTenant(tdb(c).Where("id = ? AND is_show = 1 AND delete_time IS NULL", id), c).First(&a).Error != nil {
		response.Data(c, pcArticleMissing(c, id))
		return
	}
	tdb(c).Model(&model.Article{}).Where("id = ?", a.ID).Updates(map[string]any{
		"click_actual": gorm.Expr("click_actual + 1"), "update_time": util.NowUnix(),
	})
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
	collect := userCollectsArticle(c, ctxutil.Get(c).UserID, a.ID)
	var cate model.ArticleCate
	scopeTenant(tdb(c).Where("id = ? AND delete_time IS NULL", a.Cid), c).First(&cate)
	out := articleDetailMap(c, a, a.ClickActual+a.ClickVirtual+1)
	out["last"] = last
	out["next"] = next
	out["new"] = limitArticles(c, "new", 8, int(a.Cid), int(a.ID))
	out["collect"] = collect
	out["cate_name"] = cate.Name
	response.Data(c, out)
}

func pcArticleMissing(c *gin.Context, id uint) gin.H {
	collect := userCollectsArticle(c, ctxutil.Get(c).UserID, id)
	return gin.H{
		"last": map[string]any{}, "next": map[string]any{},
		"new":     limitArticles(c, "new", 8, 0, 0),
		"collect": collect, "cate_name": nil,
	}
}

func articleDetailMap(c *gin.Context, a model.Article, click int) gin.H {
	return gin.H{
		"id": a.ID, "cid": a.Cid, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
		"image": filesvc.GetImageAttr(c, a.Image), "author": a.Author,
		"content": filesvc.RewriteContentDomains(c, a.Content),
		"is_show": a.IsShow, "sort": a.Sort, "tenant_id": a.TenantID,
		"click": click, "create_time": util.FormatDateTime(a.CreateTime),
		"update_time": util.FormatDateTimeOrNil(a.UpdateTime),
		"delete_time": util.FormatDateTimeOrNil(a.DeleteTime),
	}
}

func limitArticles(c *gin.Context, sortType string, limit, cate, exclude int) []map[string]any {
	return queryArticles(c, sortType, limit, cate, exclude, true)
}

func queryArticles(c *gin.Context, sortType string, limit, cate, exclude int, showOnly bool) []map[string]any {
	if tdb(c) == nil {
		return []map[string]any{}
	}
	db := tdb(c).Model(&model.Article{}).Where("delete_time IS NULL")
	if showOnly {
		db = db.Where("is_show = 1")
	}
	db = scopeTenant(db, c)
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
			"image": filesvc.GetImageAttr(c, a.Image), "author": a.Author,
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	return out
}

func IndexIndex(c *gin.Context) {
	var page model.DecoratePage
	db := scopeTenant(tdb(c).Where("type = 1"), c)
	_ = db.First(&page)
	articles := limitArticles(c, "new", 20, 0, 0)
	out := make([]map[string]any, 0, len(articles))
	for _, a := range articles {
		item := make(map[string]any, len(a))
		for k, v := range a {
			if k == "cid" {
				continue
			}
			item[k] = v
		}
		out = append(out, item)
	}
	response.Data(c, gin.H{
		"page":    decoratePageValue(page),
		"article": out,
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
	if !response.RequirePOST(c) {
		return
	}
	if !ratelimit.Allow(c, ratelimit.KindUpload) {
		return
	}
	// PHP UploadController::image always stores cid=0 and ignores the form field.
	name, rel, errMsg := filesvc.ReceiveUpload(c, "image", "uploads/images")
	if errMsg != "" {
		response.Fail(c, errMsg)
		return
	}
	now := util.NowUnix()
	row := model.TenantFile{
		Cid: 0, Type: 10, Name: name, URI: rel,
		Source: filesvc.SourceUser, SourceID: ctxutil.Get(c).UserID,
		TenantID: ctxutil.Get(c).TenantID, CreateTime: now, UpdateTime: util.UnixPtr(now),
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
