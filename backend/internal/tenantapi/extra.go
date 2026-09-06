package tenantapi

import (
	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/platformapi"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func AdminAll(c *gin.Context) {
	var rows []model.TenantAdmin
	db := tdb(c).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "account": a.Account})
	}
	response.Data(c, out)
}

func ArticleCateDetail(c *gin.Context) {
	var row model.ArticleCate
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")), c).First(&row).Error != nil {
		response.Fail(c, "资讯分类不存在")
		return
	}
	response.Data(c, articleCateMap(c, row))
}

func ArticleCateUpdateStatus(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "资讯分类id不能为空")
		return
	}
	var row model.ArticleCate
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")), c).First(&row).Error != nil {
		response.Fail(c, "资讯分类不存在")
		return
	}
	if msg := util.ArticleCateShowCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	scopeTID(tdb(c).Model(&model.ArticleCate{}).Where("id = ?", httpx.Uint(c, "id")), c).Update("is_show", httpx.Int(c, "is_show"))
	response.SuccessNotice(c, "修改成功")
}

func ArticleUpdateStatus(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "资讯id不能为空")
		return
	}
	var row model.Article
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.Uint(c, "id")), c).First(&row).Error != nil {
		response.Fail(c, "资讯不存在")
		return
	}
	if msg := util.ArticleCateShowCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	scopeTID(tdb(c).Model(&model.Article{}).Where("id = ?", httpx.Uint(c, "id")), c).Update("is_show", httpx.Int(c, "is_show"))
	response.SuccessNotice(c, "修改成功")
}

func ArticleAll(c *gin.Context) {
	var rows []model.Article
	db := tdb(c).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "image": filesvc.GetFileURL(c, a.Image),
		})
	}
	response.Data(c, out)
}

func DecorateDataArticle(c *gin.Context) {
	limit := httpx.Int(c, "limit")
	if limit <= 0 {
		limit = 10
	}
	var rows []model.Article
	bootstrap.DB.Where("delete_time IS NULL AND is_show = 1 AND tenant_id = 0").
		Order("id desc").Limit(limit).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "title": a.Title, "desc": a.Desc, "abstract": a.Abstract,
			"image": filesvc.GetFileURL(c, a.Image), "author": a.Author, "content": a.Content,
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	response.Success(c, "获取成功", out)
}

func DecorateDataPC(c *gin.Context) {
	var p model.DecoratePage
	q := tdb(c).Where("type = ?", 4)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	_ = q.Order("id asc").First(&p)
	update := util.FormatDateTimePtr(p.UpdateTime)
	if update == "" {
		update = util.FormatDateTime(p.CreateTime)
	}
	if update == "" {
		update = util.FormatDateTime(util.NowUnix())
	}
	response.Data(c, gin.H{"update_time": update, "pc_url": ctxutil.Domain(c) + "/pc"})
}

func DecorateTabbarSave(c *gin.Context) {
	tid := tenantDB(c)
	if style := httpx.Any(c, "style"); style != nil {
		cfgsvc.Set(c, "tabbar", "style", style)
	}
	list := httpx.Any(c, "list")
	arr, _ := list.([]any)
	if arr == nil {
		arr = httpx.List(c)
	}
	now := util.NowUnix()
	q := tdb(c)
	if tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Delete(&model.DecorateTabbar{})
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		tdb(c).Create(&model.DecorateTabbar{
			Name: util.ToString(m["name"]), Selected: filesvc.SetFileURL(c, util.ToString(m["selected"])),
			Unselected: filesvc.SetFileURL(c, util.ToString(m["unselected"])), Link: util.EncodeJSON(m["link"]),
			IsShow: util.ToInt(m["is_show"]), TenantID: tid, CreateTime: now,
		})
	}
	response.SuccessNotice(c, "操作成功")
}

func HotSearchSet(c *gin.Context) {
	status := httpx.Int(c, "status")
	if status != 1 {
		status = 0
	}
	cfgsvc.Set(c, "hot_search", "status", status)
	data := httpx.Any(c, "data")
	arr, _ := data.([]any)
	if len(arr) > 0 {
		tid := tenantDB(c)
		q := tdb(c)
		if tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Where("id > 0").Delete(&model.HotSearch{})
		now := util.NowUnix()
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			tdb(c).Create(&model.HotSearch{
				Name: util.ToString(m["name"]), Sort: util.ToInt(m["sort"]), TenantID: tid, CreateTime: now,
			})
		}
	}
	response.SuccessNotice(c, "设置成功")
}

func SettingGetCopyright(c *gin.Context) {
	response.Data(c, cfgsvc.Get(c, "copyright", "config", []any{}))
}

func SettingSetCopyright(c *gin.Context) {
	cfgsvc.Set(c, "copyright", "config", httpx.Any(c, "config"))
	response.SuccessNotice(c, "设置成功")
}

func SettingGetAgreement(c *gin.Context) {
	response.Data(c, gin.H{
		"service_title":   cfgsvc.GetString(c, "agreement", "service_title", "服务协议"),
		"service_content": cfgsvc.GetString(c, "agreement", "service_content", ""),
		"privacy_title":   cfgsvc.GetString(c, "agreement", "privacy_title", "隐私政策"),
		"privacy_content": cfgsvc.GetString(c, "agreement", "privacy_content", ""),
	})
}

func SettingSetAgreement(c *gin.Context) {
	cfgsvc.Set(c, "agreement", "service_title", httpx.Str(c, "service_title"))
	cfgsvc.Set(c, "agreement", "service_content", httpx.Str(c, "service_content"))
	cfgsvc.Set(c, "agreement", "privacy_title", httpx.Str(c, "privacy_title"))
	cfgsvc.Set(c, "agreement", "privacy_content", httpx.Str(c, "privacy_content"))
	response.SuccessNotice(c, "设置成功")
}

func SettingGetSiteStatistics(c *gin.Context) {
	response.Data(c, gin.H{"clarity_code": cfgsvc.GetString(c, "siteStatistics", "clarity_code", "")})
}

func SettingSetSiteStatistics(c *gin.Context) {
	cfgsvc.Set(c, "siteStatistics", "clarity_code", httpx.Str(c, "clarity_code"))
	response.SuccessNotice(c, "设置成功")
}

func UserAdjustMoney(c *gin.Context) {
	uid := httpx.Uint(c, "user_id")
	action := httpx.Int(c, "action")
	num := httpx.Float(c, "num")
	remark := httpx.Str(c, "remark")
	if uid == 0 {
		response.Fail(c, "请选择用户")
		return
	}
	if action != biz.INC && action != biz.DEC {
		if httpx.Str(c, "action") == "" {
			response.Fail(c, "请选择调整类型")
			return
		}
		response.Fail(c, "调整类型错误")
		return
	}
	if httpx.Str(c, "num") == "" && num == 0 {
		response.Fail(c, "请输入调整数量")
		return
	}
	if num <= 0 {
		response.Fail(c, "调整余额必须大于零")
		return
	}
	if len([]rune(remark)) > 128 {
		response.Fail(c, "备注不可超过128个符号")
		return
	}
	var user model.User
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", uid), c).First(&user).Error != nil {
		response.Fail(c, "用户不存在")
		return
	}
	err := tdb(c).Transaction(func(tx *gorm.DB) error {
		if action == biz.INC {
			if err := tx.Model(&user).Update("user_money", gorm.Expr("user_money + ?", num)).Error; err != nil {
				return err
			}
			user.UserMoney += num
			biz.AddAccountLog(tx, user.ID, user.TenantID, biz.UMIncAdmin, biz.INC, num, user.UserMoney, "", httpx.Str(c, "remark"))
			return nil
		}
		if user.UserMoney < num {
			return errInsufficient
		}
		if err := tx.Model(&user).Update("user_money", gorm.Expr("user_money - ?", num)).Error; err != nil {
			return err
		}
		user.UserMoney -= num
		biz.AddAccountLog(tx, user.ID, user.TenantID, biz.UMDecAdmin, biz.DEC, num, user.UserMoney, "", httpx.Str(c, "remark"))
		return nil
	})
	if err != nil {
		if err == errInsufficient {
			response.Fail(c, "用户可用余额仅剩"+util.MoneyString(user.UserMoney))
			return
		}
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

var errInsufficient = errString("insufficient")

type errString string

func (e errString) Error() string { return string(e) }

func GetUmChangeType(c *gin.Context) {
	response.Data(c, biz.UMChangeTypeDesc)
}

func FinanceRefundLog(c *gin.Context) {
	recordID := httpx.Uint(c, "record_id")
	var rows []model.RefundLog
	q := tdb(c).Where("record_id = ?", recordID)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Order("id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		statusText := map[int]string{0: "退款中", 1: "退款成功", 2: "退款失败"}[r.RefundStatus]
		handler := ""
		var admin model.TenantAdmin
		if r.HandleID > 0 && scopeTID(tdb(c).Where("id = ?", r.HandleID), c).First(&admin).Error == nil {
			handler = admin.Name
		}
		out = append(out, map[string]any{
			"id": r.ID, "sn": r.SN, "record_id": r.RecordID, "user_id": r.UserID,
			"handle_id": r.HandleID, "handler": handler, "order_amount": r.OrderAmount, "refund_amount": r.RefundAmount,
			"refund_status": r.RefundStatus, "refund_status_text": statusText,
			"create_time": util.FormatDateTime(r.CreateTime),
		})
	}
	response.Success(c, "", out)
}

func FinanceRefundStat(c *gin.Context) {
	db := tdb(c).Model(&model.RefundRecord{})
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	var rows []model.RefundRecord
	db.Find(&rows)
	var total, ing, success, errAmt float64
	for _, r := range rows {
		total += r.OrderAmount
		switch r.RefundStatus {
		case 0:
			ing += r.OrderAmount
		case 1:
			success += r.OrderAmount
		case 2:
			errAmt += r.OrderAmount
		}
	}
	response.Data(c, gin.H{
		"total": round2(total), "ing": round2(ing), "success": round2(success), "error": round2(errAmt),
	})
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func RechargeRefund(c *gin.Context) {
	if _, ok := httpx.Params(c)["recharge_id"]; !ok {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.Uint(c, "recharge_id")
	var order model.RechargeOrder
	oq := tdb(c).Where("id = ? AND delete_time IS NULL", id)
	if tid := tenantDB(c); tid > 0 {
		oq = oq.Where("tenant_id = ?", tid)
	}
	if oq.First(&order).Error != nil {
		response.Fail(c, "充值订单不存在")
		return
	}
	if order.PayStatus != 1 {
		response.Fail(c, "当前订单不可退款")
		return
	}
	if order.RefundStatus == 1 {
		response.Fail(c, "订单已发起退款,退款失败请到退款记录重新退款")
		return
	}
	udb := tenantdb.ForTenant(order.TenantID)
	adminID := ctxutil.Get(c).AdminID
	var rec model.RefundRecord
	var user model.User
	err := udb.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Update("refund_status", 1).Error; err != nil {
			return err
		}
		uq := tx.Model(&model.User{}).Where("id = ?", order.UserID)
		if order.TenantID > 0 {
			uq = uq.Where("tenant_id = ?", order.TenantID)
		}
		if err := uq.Updates(map[string]any{
			"user_money":            gorm.Expr("user_money - ?", order.OrderAmount),
			"total_recharge_amount": gorm.Expr("total_recharge_amount - ?", order.OrderAmount),
		}).Error; err != nil {
			return err
		}
		tx.First(&user, order.UserID)
		biz.AddAccountLog(tx, order.UserID, order.TenantID, biz.UMIncAdmin, biz.DEC, order.OrderAmount, user.UserMoney, order.SN, "充值订单退款")
		exists := func(sn string) bool {
			var n int64
			tx.Model(&model.RefundRecord{}).Where("sn = ?", sn).Count(&n)
			return n > 0
		}
		way := 2
		if order.PayWay == 2 || order.PayWay == 3 {
			way = 1
		}
		rec = model.RefundRecord{
			SN: util.GenerateSN(exists, "", 4), UserID: order.UserID, OrderID: order.ID, OrderSN: order.SN,
			OrderType: "recharge", OrderAmount: order.OrderAmount, RefundAmount: order.OrderAmount,
			RefundType: 1, TransactionID: order.TransactionID, RefundWay: way, RefundStatus: 0,
			TenantID: order.TenantID, CreateTime: util.NowUnix(),
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		logExists := func(sn string) bool {
			var n int64
			tx.Model(&model.RefundLog{}).Where("sn = ?", sn).Count(&n)
			return n > 0
		}
		return tx.Create(&model.RefundLog{
			SN: util.GenerateSN(logExists, "", 4), RecordID: rec.ID, UserID: order.UserID, HandleID: adminID,
			OrderAmount: order.OrderAmount, RefundAmount: order.OrderAmount, RefundStatus: 0,
			RefundMsg: "后台退款", TenantID: order.TenantID, CreateTime: util.NowUnix(),
		}).Error
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	if order.PayWay != 2 && order.PayWay != 3 {
		refundFailHandle(c, rec.ID, 0, "支付方式异常")
		response.Fail(c, "支付方式异常")
		return
	}
	if err := remoteRefund(c, &order, lastRefundLogSN(c, rec.ID), rec.ID); err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func lastRefundLogSN(c *gin.Context, recID uint) string {
	var last model.RefundLog
	q := tdb(c).Where("record_id = ?", recID)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.Order("id desc").First(&last).Error == nil {
		return last.SN
	}
	return ""
}

func refundFailHandle(c *gin.Context, recID, logID uint, msg string) {
	if recID == 0 {
		return
	}
	rq := tdb(c).Model(&model.RefundRecord{}).Where("id = ?", recID)
	if tid := tenantDB(c); tid > 0 {
		rq = rq.Where("tenant_id = ?", tid)
	}
	rq.Update("refund_status", 2)
	q := tdb(c).Model(&model.RefundLog{}).Where("record_id = ?", recID)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if logID > 0 {
		q = q.Where("id = ?", logID)
	} else {
		var last model.RefundLog
		lq := tdb(c).Where("record_id = ?", recID)
		if tid := tenantDB(c); tid > 0 {
			lq = lq.Where("tenant_id = ?", tid)
		}
		if lq.Order("id desc").First(&last).Error == nil {
			q = tdb(c).Model(&model.RefundLog{}).Where("id = ?", last.ID)
			if tid := tenantDB(c); tid > 0 {
				q = q.Where("tenant_id = ?", tid)
			}
		}
	}
	q.Updates(map[string]any{"refund_status": 2, "refund_msg": msg})
}

func applyAliRefundSuccess(c *gin.Context, order *model.RechargeOrder, recID uint, res pay.AliRefundResult) {
	msg := ""
	if res.Raw != nil {
		msg = util.EncodeJSON(res.Raw)
	}
	rq := tdb(c).Model(&model.RefundRecord{}).Where("id = ?", recID)
	if tid := tenantDB(c); tid > 0 {
		rq = rq.Where("tenant_id = ?", tid)
	}
	rq.Update("refund_status", 1)
	var last model.RefundLog
	lq := tdb(c).Where("record_id = ?", recID)
	if tid := tenantDB(c); tid > 0 {
		lq = lq.Where("tenant_id = ?", tid)
	}
	if lq.Order("id desc").First(&last).Error == nil {
		tdb(c).Model(&model.RefundLog{}).Where("id = ?", last.ID).Updates(map[string]any{
			"refund_status": 1, "refund_msg": msg,
		})
	}
	if order != nil && recID > 0 {
		tdb(c).Model(order).Update("refund_transaction_id", res.TradeNo)
	}
}

func remoteRefund(c *gin.Context, order *model.RechargeOrder, refundSN string, recID uint) error {
	if refundSN == "" {
		return nil
	}
	var err error
	switch order.PayWay {
	case 2:
		err = pay.WechatRefundByTenant(order.TenantID, order.TransactionID, refundSN, order.OrderAmount, order.OrderAmount)
	case 3:
		var res pay.AliRefundResult
		res, err = pay.AliRefundByTenant(order.TenantID, order.SN, refundSN, order.OrderAmount)
		if err == nil && res.OK {
			applyAliRefundSuccess(c, order, recID, res)
		}
	}
	if err != nil {
		refundFailHandle(c, recID, 0, err.Error())
		return err
	}
	return nil
}

func RechargeRefundAgain(c *gin.Context) {
	if _, ok := httpx.Params(c)["record_id"]; !ok {
		response.Fail(c, "参数缺失")
		return
	}
	var rec model.RefundRecord
	rq := tdb(c).Where("id = ?", httpx.Uint(c, "record_id"))
	if tid := tenantDB(c); tid > 0 {
		rq = rq.Where("tenant_id = ?", tid)
	}
	if rq.First(&rec).Error != nil {
		response.Fail(c, "退款记录不存在")
		return
	}
	if rec.RefundStatus == 1 {
		response.Fail(c, "该退款记录已退款成功")
		return
	}
	var againOrder model.RechargeOrder
	oq := tdb(c).Where("id = ?", rec.OrderID)
	if tid := tenantDB(c); tid > 0 {
		oq = oq.Where("tenant_id = ?", tid)
	}
	oq.First(&againOrder)
	againLog := model.RefundLog{
		SN: util.GenerateSN(func(sn string) bool {
			var n int64
			tdb(c).Model(&model.RefundLog{}).Where("sn = ?", sn).Count(&n)
			return n > 0
		}, "", 4),
		RecordID: rec.ID, UserID: rec.UserID, HandleID: ctxutil.Get(c).AdminID,
		OrderAmount: rec.OrderAmount, RefundAmount: rec.RefundAmount, RefundStatus: 0,
		RefundMsg: "重新退款", TenantID: rec.TenantID, CreateTime: util.NowUnix(),
	}
	tdb(c).Create(&againLog)
	if againOrder.PayWay != 2 && againOrder.PayWay != 3 {
		refundFailHandle(c, rec.ID, 0, "支付方式异常")
		response.Fail(c, "支付方式异常")
		return
	}
	if err := remoteRefund(c, &againOrder, againLog.SN, rec.ID); err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func OAReplyLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.OfficialAccountReply{}).Where("delete_time IS NULL")
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if t := lists.ParamInt(q, "reply_type"); t > 0 {
		db = db.Where("reply_type = ?", t)
	}
	var count int64
	db.Count(&count)
	var rows []model.OfficialAccountReply
	db.Order("sort desc, id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, oaReplyListMap(row))
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func OAReplyAdd(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.OAReplyWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if httpx.Int(c, "reply_type") == 2 && httpx.Int(c, "sort") < 0 {
		response.Fail(c, "排序值须大于或等于0")
		return
	}
	row := model.OfficialAccountReply{
		TenantID: tenantDB(c), Name: httpx.Str(c, "name"), Keyword: httpx.Str(c, "keyword"),
		ReplyType: httpx.Int(c, "reply_type"), MatchingType: httpx.Int(c, "matching_type"),
		ContentType: httpx.Int(c, "content_type"), Content: httpx.Str(c, "content"),
		Status: httpx.Int(c, "status"), Sort: httpx.Int(c, "sort"), CreateTime: util.NowUnix(),
	}
	if row.MatchingType == 0 {
		row.MatchingType = 1
	}
	if row.ContentType == 0 {
		row.ContentType = 1
	}
	if row.ReplyType != 2 && row.Status == 1 {
		q := tdb(c).Model(&model.OfficialAccountReply{}).Where("reply_type = ? AND delete_time IS NULL", row.ReplyType)
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Update("status", 0)
	}
	tdb(c).Create(&row)
	response.SuccessNotice(c, "操作成功")
}

func OAReplyEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.OAReplyWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	replyType := httpx.Int(c, "reply_type")
	status := httpx.Int(c, "status")
	if replyType != 2 && status == 1 {
		q := tdb(c).Model(&model.OfficialAccountReply{}).Where("reply_type = ? AND id <> ? AND delete_time IS NULL", replyType, httpx.Uint(c, "id"))
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Update("status", 0)
	}
	q := tdb(c).Model(&model.OfficialAccountReply{}).Where("id = ?", httpx.Uint(c, "id"))
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Updates(map[string]any{
		"name": httpx.Str(c, "name"), "keyword": httpx.Str(c, "keyword"),
		"reply_type": replyType, "matching_type": httpx.Int(c, "matching_type"),
		"content_type": httpx.Int(c, "content_type"), "content": httpx.Str(c, "content"),
		"status": status, "sort": httpx.Int(c, "sort"), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "操作成功")
}

func oaReplyByID(c *gin.Context, id uint) (model.OfficialAccountReply, bool) {
	var row model.OfficialAccountReply
	q := tdb(c).Where("id = ? AND delete_time IS NULL", id)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&row).Error != nil || row.ID == 0 {
		return row, false
	}
	return row, true
}

func OAReplyDelete(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	if _, ok := oaReplyByID(c, httpx.Uint(c, "id")); !ok {
		response.Fail(c, "记录不存在")
		return
	}
	q := tdb(c).Unscoped().Where("id = ?", httpx.Uint(c, "id"))
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Delete(&model.OfficialAccountReply{})
	response.SuccessNotice(c, "操作成功")
}

func OAReplyDetail(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	row, ok := oaReplyByID(c, httpx.Uint(c, "id"))
	if !ok {
		response.Fail(c, "记录不存在")
		return
	}
	response.Data(c, oaReplyDetailMap(row))
}

func oaReplyListMap(row model.OfficialAccountReply) map[string]any {
	return map[string]any{
		"id": row.ID, "name": row.Name, "keyword": row.Keyword,
		"matching_type": row.MatchingType, "content": row.Content, "content_type": row.ContentType,
		"status": row.Status, "sort": row.Sort,
		"matching_type_desc": row.MatchingType, "content_type_desc": row.ContentType, "status_desc": row.Status,
	}
}

func oaReplyDetailMap(row model.OfficialAccountReply) map[string]any {
	return map[string]any{
		"id": row.ID, "name": row.Name, "keyword": row.Keyword,
		"reply_type": row.ReplyType, "matching_type": row.MatchingType,
		"content_type": row.ContentType, "content": row.Content,
		"status": row.Status, "sort": row.Sort,
		"reply_type_desc": row.ReplyType, "matching_type_desc": row.MatchingType,
		"content_type_desc": row.ContentType, "status_desc": row.Status,
	}
}

func OAReplyStatus(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	row, ok := oaReplyByID(c, httpx.Uint(c, "id"))
	if !ok {
		response.Fail(c, "记录不存在")
		return
	}
	status := 0
	if row.Status == 0 {
		status = 1
	}
	if row.ReplyType != 2 && status == 1 {
		q := tdb(c).Model(&model.OfficialAccountReply{}).Where("reply_type = ? AND id <> ? AND delete_time IS NULL", row.ReplyType, row.ID)
		if tid := tenantDB(c); tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Update("status", 0)
	}
	q := tdb(c).Model(&model.OfficialAccountReply{}).Where("id = ?", row.ID)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Update("status", status)
	response.SuccessNotice(c, "操作成功")
}

func OAReplySort(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	if msg := util.OAReplySortCheck(httpx.Params(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	sort := httpx.Int(c, "new_sort")
	q := tdb(c).Model(&model.OfficialAccountReply{}).Where("id = ?", httpx.Uint(c, "id"))
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Updates(map[string]any{
		"sort": sort, "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "操作成功")
}

func OAMenuDetail(c *gin.Context) {
	data := cfgsvc.Get(c, "oa_setting", "menu", []any{})
	response.Data(c, util.CoerceOAMenuHasMenu(data))
}

func OAMenuSave(c *gin.Context) {
	menu := httpx.List(c)
	if menu == nil {
		if v := httpx.Any(c, "menu"); v != nil {
			if arr, ok := v.([]any); ok {
				menu = arr
			}
		}
	}
	if menu == nil {
		response.Fail(c, "请设置正确格式菜单")
		return
	}
	if msg := util.OAMenuCheck(menu); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "oa_setting", "menu", menu)
	response.SuccessNotice(c, "保存成功")
}

func OAMenuSaveAndPublish(c *gin.Context) {
	menu := httpx.List(c)
	if menu == nil {
		if v := httpx.Any(c, "menu"); v != nil {
			if arr, ok := v.([]any); ok {
				menu = arr
			}
		}
	}
	if menu == nil {
		response.Fail(c, "请设置正确格式菜单")
		return
	}
	if msg := util.OAMenuCheck(menu); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "oa_setting", "menu", menu)
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先完成微信公众号配置")
		return
	}
	if err := wechat.PublishMenu(appID, secret, menu); err != nil {
		response.Fail(c, "保存成功但发布失败："+err.Error())
		return
	}
	response.Success(c, "保存并发布成功", nil)
}

func TenantNoticeLists(c *gin.Context) {
	q := lists.Parse(c)
	db := tdb(c).Model(&model.TenantNoticeSetting{})
	if tid := tenantDB(c); tid > 0 {
		db = db.Where("tenant_id = ?", tid)
	}
	if lists.Param(q, "recipient") != "" {
		db = db.Where("recipient = ?", lists.ParamInt(q, "recipient"))
	}
	if lists.Param(q, "type") != "" {
		db = db.Where("type = ?", lists.ParamInt(q, "type"))
	}
	var count int64
	db.Count(&count)
	var rows []model.TenantNoticeSetting
	db.Order("id asc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		sms := util.DecodeJSON(r.SmsNotice)
		smsStatus := "停用"
		if m, ok := sms.(map[string]any); ok && util.ToInt(m["status"]) == 1 {
			smsStatus = "启用"
		}
		typeDesc := "业务通知"
		if r.Type == 2 {
			typeDesc = "验证码"
		}
		out = append(out, map[string]any{
			"id": r.ID, "scene_name": r.SceneName, "sms_notice": sms, "type": r.Type,
			"sms_status_desc": smsStatus, "type_desc": typeDesc,
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func tenantNoticeByID(c *gin.Context, id uint) (model.TenantNoticeSetting, bool) {
	var r model.TenantNoticeSetting
	if id == 0 {
		return r, false
	}
	q := tdb(c).Where("id = ?", id)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.First(&r).Error != nil || r.ID == 0 {
		return r, false
	}
	return r, true
}

func TenantNoticeDetail(c *gin.Context) {
	id := httpx.Uint(c, "id")
	if id == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	r, ok := tenantNoticeByID(c, id)
	if !ok {
		response.Data(c, []any{})
		return
	}
	response.Data(c, biz.FormatNoticeDetail(
		r.ID, r.Type, r.SceneID, r.SceneName, r.SceneDesc,
		r.SystemNotice, r.SmsNotice, r.OaNotice, r.MnpNotice, r.Support,
	))
}

func TenantNoticeSet(c *gin.Context) {
	id := httpx.Uint(c, "id")
	_, exists := tenantNoticeByID(c, id)
	updates, err := biz.ApplyNoticeSet(exists, id, httpx.Any(c, "template"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	q := tdb(c).Model(&model.TenantNoticeSetting{}).Where("id = ?", id)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	q.Updates(updates)
	response.SuccessNotice(c, "设置成功")
}

func SettingUserGetConfig(c *gin.Context)   { platformapi.UserGetConfig(c) }
func SettingUserSetConfig(c *gin.Context)   { platformapi.UserSetConfig(c) }
func SettingUserGetRegister(c *gin.Context) { platformapi.UserGetRegisterConfig(c) }
func SettingUserSetRegister(c *gin.Context) { platformapi.UserSetRegisterConfig(c) }
func SettingTransactionGet(c *gin.Context)  { platformapi.TransactionGet(c) }
func SettingTransactionSet(c *gin.Context)  { platformapi.TransactionSet(c) }
func SettingCustomerGet(c *gin.Context)     { platformapi.CustomerGet(c) }
func SettingCustomerSet(c *gin.Context)     { platformapi.CustomerSet(c) }
