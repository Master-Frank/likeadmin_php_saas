package tenantapi

import (
	"fmt"
	"strings"

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
	tid, ok := requireTenant(c)
	if !ok {
		response.Data(c, []any{})
		return
	}
	var rows []model.TenantAdmin
	db := tdb(c).Where("delete_time IS NULL AND tenant_id = ?", tid)
	db.Order("id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{"id": a.ID, "name": a.Name, "account": a.Account})
	}
	response.Data(c, out)
}

func ArticleCateDetail(c *gin.Context) {
	if httpx.QueryUint(c, "id") == 0 {
		response.Fail(c, "资讯分类id不能为空")
		return
	}
	var row model.ArticleCate
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.QueryUint(c, "id")), c).First(&row).Error != nil {
		response.Fail(c, "资讯分类不存在")
		return
	}
	response.Data(c, articleCateRaw(row))
}

func ArticleCateUpdateStatus(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "资讯分类id不能为空")
		return
	}
	var row model.ArticleCate
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "id")), c).First(&row).Error != nil {
		response.Fail(c, "资讯分类不存在")
		return
	}
	if msg := util.ArticleCateShowCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	scopeTID(tdb(c).Model(&model.ArticleCate{}).Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "id")), c).Updates(map[string]any{
		"is_show": httpx.BodyInt(c, "is_show"), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "修改成功")
}

func ArticleUpdateStatus(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if !httpx.BodyIDPresent(c) {
		response.Fail(c, "资讯id不能为空")
		return
	}
	var row model.Article
	if scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "id")), c).First(&row).Error != nil {
		response.Fail(c, "资讯不存在")
		return
	}
	if msg := util.ArticleCateShowCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	scopeTID(tdb(c).Model(&model.Article{}).Where("id = ? AND delete_time IS NULL", httpx.BodyUint(c, "id")), c).Updates(map[string]any{
		"is_show": httpx.BodyInt(c, "is_show"), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "修改成功")
}

func ArticleAll(c *gin.Context) {
	tid, ok := requireTenant(c)
	if !ok {
		response.Data(c, []any{})
		return
	}
	var rows []model.Article
	db := tdb(c).Where("delete_time IS NULL AND tenant_id = ?", tid)
	db.Order("sort desc, id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id": a.ID, "cid": a.Cid, "title": a.Title, "image": filesvc.GetImageAttr(c, a.Image),
		})
	}
	response.Data(c, out)
}

func DecorateDataArticle(c *gin.Context) {
	limit := httpx.QueryInt(c, "limit")
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
			"image": filesvc.GetImageAttr(c, a.Image), "author": a.Author, "content": filesvc.RewriteContentDomains(c, a.Content),
			"click": a.ClickActual + a.ClickVirtual, "create_time": util.FormatDateTime(a.CreateTime),
		})
	}
	response.Success(c, "获取成功", out)
}

func DecorateDataPC(c *gin.Context) {
	var p model.DecoratePage
	_ = scopeTID(tdb(c).Where("id = ?", 4), c).First(&p)
	update := ""
	if p.ID > 0 {
		update = util.FormatDateTimePtr(p.UpdateTime)
	}
	if update == "" {
		update = util.FormatDateTime(util.NowUnix())
	}
	response.Data(c, gin.H{"update_time": update, "pc_url": ctxutil.Domain(c) + "/pc"})
}

func DecorateTabbarSave(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	tid := tenantDB(c)
	if style := httpx.BodyAny(c, "style"); style != nil {
		cfgsvc.Set(c, "tabbar", "style", style)
	}
	list := httpx.BodyAny(c, "list")
	arr, _ := list.([]any)
	if arr == nil {
		arr = httpx.List(c)
	}
	now := util.NowUnix()
	tdb(c).Where("tenant_id = ?", tid).Delete(&model.DecorateTabbar{})
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		tdb(c).Create(&model.DecorateTabbar{
			Name: util.ToString(m["name"]), Selected: filesvc.SetFileURL(c, util.ToString(m["selected"])),
			Unselected: filesvc.SetFileURL(c, util.ToString(m["unselected"])), Link: util.EncodeJSON(m["link"]),
			IsShow: util.ToInt(m["is_show"]), TenantID: tid, CreateTime: now, UpdateTime: util.UnixPtr(now),
		})
	}
	response.SuccessNotice(c, "操作成功")
}

func HotSearchSet(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !requirePlatformTenant(c) {
		return
	}
	// PHP: empty($params['status']) ? 0 : $params['status'] — keep values like 2.
	status := 0
	if httpx.BodyHas(c, "status") && strings.TrimSpace(httpx.BodyStr(c, "status")) != "" && httpx.BodyInt(c, "status") != 0 {
		status = httpx.BodyInt(c, "status")
	}
	cfgsvc.Set(c, "hot_search", "status", status)
	data := httpx.BodyAny(c, "data")
	arr, _ := data.([]any)
	if len(arr) > 0 {
		tid, ok := requireTenant(c)
		if !ok {
			response.Fail(c, "参数缺失")
			return
		}
		tdb(c).Where("tenant_id = ? AND id > 0", tid).Delete(&model.HotSearch{})
		now := util.NowUnix()
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			row := model.HotSearch{
				Name: util.ToString(m["name"]), Sort: util.ToInt(m["sort"]), TenantID: tid, CreateTime: now,
			}
			if id := uint(util.ToInt(m["id"])); id > 0 {
				row.ID = id
			}
			tdb(c).Create(&row)
		}
	}
	response.SuccessNotice(c, "设置成功")
}

func SettingGetCopyright(c *gin.Context) {
	response.Data(c, cfgsvc.Get(c, "copyright", "config", []any{}))
}

func SettingSetCopyright(c *gin.Context) {
	if !requirePlatformTenant(c) {
		return
	}
	cfg := httpx.BodyAny(c, "config")
	if msg := util.CopyrightConfigCheck(cfg); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "copyright", "config", cfg)
	response.SuccessNotice(c, "设置成功")
}

func SettingGetAgreement(c *gin.Context) {
	response.Data(c, gin.H{
		"service_title":   cfgsvc.GetString(c, "agreement", "service_title", ""),
		"service_content": filesvc.RewriteContentDomains(c, cfgsvc.GetString(c, "agreement", "service_content", "")),
		"privacy_title":   cfgsvc.GetString(c, "agreement", "privacy_title", ""),
		"privacy_content": filesvc.RewriteContentDomains(c, cfgsvc.GetString(c, "agreement", "privacy_content", "")),
	})
}

func SettingSetAgreement(c *gin.Context) {
	if !requirePlatformTenant(c) {
		return
	}
	cfgsvc.Set(c, "agreement", "service_title", httpx.BodyStr(c, "service_title"))
	cfgsvc.Set(c, "agreement", "service_content", filesvc.ClearContentDomains(c, httpx.BodyStr(c, "service_content")))
	cfgsvc.Set(c, "agreement", "privacy_title", httpx.BodyStr(c, "privacy_title"))
	cfgsvc.Set(c, "agreement", "privacy_content", filesvc.ClearContentDomains(c, httpx.BodyStr(c, "privacy_content")))
	response.SuccessNotice(c, "设置成功")
}

func SettingGetSiteStatistics(c *gin.Context) {
	response.Data(c, gin.H{"clarity_code": cfgsvc.GetString(c, "siteStatistics", "clarity_code", "")})
}

func SettingSetSiteStatistics(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !requirePlatformTenant(c) {
		return
	}
	cfgsvc.Set(c, "siteStatistics", "clarity_code", httpx.BodyStr(c, "clarity_code"))
	response.SuccessNotice(c, "设置成功")
}

func UserAdjustMoney(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if !httpx.BodyPresent(c, "user_id") {
		// PHP AdjustUserMoney rule key is user_id; ThinkPHP prints "user_id不能为空".
		response.Fail(c, "user_id不能为空")
		return
	}
	uid := httpx.BodyUint(c, "user_id")
	action := httpx.BodyInt(c, "action")
	num := httpx.BodyFloat(c, "num")
	remark := httpx.BodyStr(c, "remark")
	if action != biz.INC && action != biz.DEC {
		if httpx.BodyStr(c, "action") == "" {
			response.Fail(c, "请选择调整类型")
			return
		}
		response.Fail(c, "调整类型错误")
		return
	}
	if httpx.BodyStr(c, "num") == "" && num == 0 {
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
			if err := tx.Model(&user).Updates(map[string]any{
				"user_money": gorm.Expr("user_money + ?", num), "update_time": util.NowUnix(),
			}).Error; err != nil {
				return err
			}
			user.UserMoney += num
			biz.AddAccountLog(tx, user.ID, user.TenantID, biz.UMIncAdmin, biz.INC, num, user.UserMoney, "", httpx.BodyStr(c, "remark"))
			return nil
		}
		if user.UserMoney < num {
			return errInsufficient
		}
		if err := tx.Model(&user).Updates(map[string]any{
			"user_money": gorm.Expr("user_money - ?", num), "update_time": util.NowUnix(),
		}).Error; err != nil {
			return err
		}
		user.UserMoney -= num
		biz.AddAccountLog(tx, user.ID, user.TenantID, biz.UMDecAdmin, biz.DEC, num, user.UserMoney, "", httpx.BodyStr(c, "remark"))
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
	recordID := httpx.QueryUint(c, "record_id")
	// PHP RefundLogic::refundLog queries by record_id with no existence check.
	var rows []model.RefundLog
	q := scopeTID(tdb(c).Where("record_id = ?", recordID), c)
	q.Order("id desc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		statusText := map[int]string{0: "退款中", 1: "退款成功", 2: "退款失败"}[r.RefundStatus]
		handler := refundHandlerName(c, r.HandleID)
		out = append(out, map[string]any{
			"id": r.ID, "sn": r.SN, "record_id": r.RecordID, "user_id": r.UserID,
			"handle_id": r.HandleID, "handler": handler,
			"order_amount": util.MoneyString(r.OrderAmount), "refund_amount": util.MoneyString(r.RefundAmount),
			"refund_status": r.RefundStatus, "refund_status_text": statusText,
			"tenant_id": r.TenantID, "create_time": util.FormatDateTime(r.CreateTime),
			"update_time": util.FormatDateTimeOrNil(r.UpdateTime),
		})
	}
	response.SuccessSilent(c, "", out)
}

// refundHandlerName prefers the current tenant admin (who actually issues tenant
// refunds), then falls back to platform la_admin like PHP RefundLog::getHandlerAttr.
func refundHandlerName(c *gin.Context, handleID uint) string {
	if handleID == 0 {
		return ""
	}
	var tenantAdmin model.TenantAdmin
	if scopeTID(tdb(c).Where("id = ?", handleID), c).First(&tenantAdmin).Error == nil && tenantAdmin.Name != "" {
		return tenantAdmin.Name
	}
	if bootstrap.DB == nil {
		return ""
	}
	var admin model.Admin
	if bootstrap.DB.Where("id = ? AND delete_time IS NULL", handleID).First(&admin).Error == nil {
		return admin.Name
	}
	return ""
}

func FinanceRefundStat(c *gin.Context) {
	tid, ok := requireTenant(c)
	if !ok {
		response.Data(c, gin.H{"total": 0, "ing": 0, "success": 0, "error": 0})
		return
	}
	db := tdb(c).Model(&model.RefundRecord{}).Where("tenant_id = ?", tid)
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

func rechargeUserMoneyEnough(db *gorm.DB, userID, tenantID uint, amount float64) bool {
	if db == nil {
		return false
	}
	var user model.User
	q := db.Where("id = ? AND delete_time IS NULL AND tenant_id = ?", userID, tenantID)
	if q.First(&user).Error != nil {
		return false
	}
	return user.UserMoney >= amount
}

func RechargeRefund(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if !httpx.BodyHas(c, "recharge_id") {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.BodyUint(c, "recharge_id")
	var order model.RechargeOrder
	oq := scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", id), c)
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
	if order.OrderAmount <= 0 {
		// PHP RefundLogic::refundBeforeCheck throws before any writes; the outer txn rolls back.
		response.Fail(c, "订单金额异常")
		return
	}
	if !rechargeUserMoneyEnough(udb, order.UserID, order.TenantID, order.OrderAmount) {
		response.Fail(c, "退款失败:用户余额已不足退款金额")
		return
	}
	if bootstrap.DB == nil {
		response.Fail(c, "系统错误")
		return
	}
	adminID := ctxutil.Get(c).AdminID
	var rec model.RefundRecord
	var user model.User
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		now := util.NowUnix()
		if err := tx.Model(&order).Updates(map[string]any{"refund_status": 1, "update_time": now}).Error; err != nil {
			return err
		}
		userDB := tenantdb.ForTenantOn(tx, order.TenantID)
		uq := userDB.Model(&model.User{}).Where("id = ? AND tenant_id = ? AND delete_time IS NULL", order.UserID, order.TenantID)
		if err := uq.Updates(map[string]any{
			"user_money":            gorm.Expr("user_money - ?", order.OrderAmount),
			"total_recharge_amount": gorm.Expr("total_recharge_amount - ?", order.OrderAmount),
			"update_time":           now,
		}).Error; err != nil {
			return err
		}
		userDB.Where("id = ? AND tenant_id = ? AND delete_time IS NULL", order.UserID, order.TenantID).First(&user)
		biz.AddAccountLog(userDB, order.UserID, order.TenantID, biz.UMIncAdmin, biz.DEC, order.OrderAmount, user.UserMoney, order.SN, "充值订单退款")
		exists := func(sn string) bool {
			var n int64
			q := tx.Model(&model.RefundRecord{}).Where("sn = ? AND tenant_id = ?", sn, order.TenantID)
			q.Count(&n)
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
			TenantID: order.TenantID, CreateTime: now, UpdateTime: util.UnixPtr(now),
		}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		logExists := func(sn string) bool {
			var n int64
			q := tx.Model(&model.RefundLog{}).Where("sn = ? AND tenant_id = ?", sn, order.TenantID)
			q.Count(&n)
			return n > 0
		}
		return tx.Create(&model.RefundLog{
			SN: util.GenerateSN(logExists, "", 4), RecordID: rec.ID, UserID: order.UserID, HandleID: adminID,
			OrderAmount: order.OrderAmount, RefundAmount: order.OrderAmount, RefundStatus: 0,
			TenantID: order.TenantID, CreateTime: now, UpdateTime: util.UnixPtr(now),
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
	q := scopeTID(tdb(c).Where("record_id = ?", recID), c)
	if q.Order("id desc").First(&last).Error == nil {
		return last.SN
	}
	return ""
}

func refundFailHandle(c *gin.Context, recID, logID uint, msg string) {
	if recID == 0 {
		return
	}
	now := util.NowUnix()
	rq := scopeTID(tdb(c).Model(&model.RefundRecord{}).Where("id = ?", recID), c)
	rq.Updates(map[string]any{"refund_status": 2, "update_time": now})
	q := scopeTID(tdb(c).Model(&model.RefundLog{}).Where("record_id = ?", recID), c)
	if logID > 0 {
		q = q.Where("id = ?", logID)
	} else {
		var last model.RefundLog
		lq := scopeTID(tdb(c).Where("record_id = ?", recID), c)
		if lq.Order("id desc").First(&last).Error == nil {
			q = scopeTID(tdb(c).Model(&model.RefundLog{}).Where("id = ?", last.ID), c)
		}
	}
	q.Updates(map[string]any{"refund_status": 2, "refund_msg": msg, "update_time": now})
}

func applyAliRefundSuccess(c *gin.Context, order *model.RechargeOrder, recID uint, res pay.AliRefundResult) {
	msg := ""
	if res.Raw != nil {
		msg = util.EncodeJSON(res.Raw)
	}
	now := util.NowUnix()
	rq := scopeTID(tdb(c).Model(&model.RefundRecord{}).Where("id = ?", recID), c)
	rq.Updates(map[string]any{"refund_status": 1, "update_time": now})
	var last model.RefundLog
	lq := scopeTID(tdb(c).Where("record_id = ?", recID), c)
	if lq.Order("id desc").First(&last).Error == nil {
		uq := scopeTID(tdb(c).Model(&model.RefundLog{}).Where("id = ?", last.ID), c)
		uq.Updates(map[string]any{
			"refund_status": 1, "refund_msg": msg, "update_time": now,
		})
	}
	if order != nil && recID > 0 {
		tdb(c).Model(order).Updates(map[string]any{"refund_transaction_id": res.TradeNo, "update_time": now})
	}
}

func remoteRefund(c *gin.Context, order *model.RechargeOrder, refundSN string, recID uint) error {
	if refundSN == "" {
		return nil
	}
	if order == nil || order.OrderAmount <= 0 {
		err := fmt.Errorf("订单金额异常")
		refundFailHandle(c, recID, 0, err.Error())
		return err
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
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if !httpx.BodyHas(c, "record_id") {
		response.Fail(c, "参数缺失")
		return
	}
	var rec model.RefundRecord
	rq := scopeTID(tdb(c).Where("id = ?", httpx.BodyUint(c, "record_id")), c)
	if rq.First(&rec).Error != nil {
		response.Fail(c, "退款记录不存在")
		return
	}
	if rec.RefundStatus == 1 {
		response.Fail(c, "该退款记录已退款成功")
		return
	}
	var againOrder model.RechargeOrder
	oq := scopeTID(tdb(c).Where("id = ? AND delete_time IS NULL", rec.OrderID), c)
	oq.First(&againOrder)
	if againOrder.OrderAmount <= 0 {
		response.Fail(c, "订单金额异常")
		return
	}
	if !rechargeUserMoneyEnough(tenantdb.ForTenant(rec.TenantID), rec.UserID, rec.TenantID, againOrder.OrderAmount) {
		response.Fail(c, "退款失败:用户余额已不足退款金额")
		return
	}
	now := util.NowUnix()
	againLog := model.RefundLog{
		SN: util.GenerateSN(func(sn string) bool {
			var n int64
			q := tdb(c).Model(&model.RefundLog{}).Where("sn = ? AND tenant_id = ?", sn, rec.TenantID)
			q.Count(&n)
			return n > 0
		}, "", 4),
		RecordID: rec.ID, UserID: rec.UserID, HandleID: ctxutil.Get(c).AdminID,
		OrderAmount: rec.OrderAmount, RefundAmount: rec.RefundAmount, RefundStatus: 0,
		TenantID: rec.TenantID, CreateTime: now, UpdateTime: util.UnixPtr(now),
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
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	tid, ok := requireTenant(c)
	if !ok {
		response.Lists(c, []map[string]any{}, 0, q.PageNo, q.PageSize, nil)
		return
	}
	db := tdb(c).Model(&model.OfficialAccountReply{}).Where("delete_time IS NULL AND tenant_id = ?", tid)
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
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.OAReplyWriteCheck(p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	if httpx.BodyInt(c, "reply_type") == 2 && httpx.BodyInt(c, "sort") < 0 {
		response.Fail(c, "排序值须大于或等于0")
		return
	}
	now := util.NowUnix()
	row := model.OfficialAccountReply{
		TenantID: tenantDB(c), Name: httpx.BodyStr(c, "name"), Keyword: httpx.BodyStr(c, "keyword"),
		ReplyType: httpx.BodyInt(c, "reply_type"), MatchingType: httpx.BodyInt(c, "matching_type"),
		ContentType: httpx.BodyInt(c, "content_type"), Content: httpx.BodyStr(c, "content"),
		Status: httpx.BodyInt(c, "status"), Sort: httpx.BodyInt(c, "sort"), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if row.TenantID == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	if row.MatchingType == 0 {
		row.MatchingType = 1
	}
	if row.ContentType == 0 {
		row.ContentType = 1
	}
	if row.ReplyType != 2 && row.Status == 1 {
		tdb(c).Model(&model.OfficialAccountReply{}).
			Where("reply_type = ? AND tenant_id = ? AND delete_time IS NULL", row.ReplyType, row.TenantID).
			Update("status", 0)
	}
	tdb(c).Create(&row)
	response.SuccessNotice(c, "操作成功")
}

func OAReplyEdit(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	p := httpx.Body(c)
	if msg := util.OAReplyWriteCheck(p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	if httpx.BodyInt(c, "reply_type") == 2 && httpx.BodyInt(c, "sort") < 0 {
		response.Fail(c, "排序值须大于或等于0")
		return
	}
	tid, ok := requireTenant(c)
	if !ok {
		response.Fail(c, "参数缺失")
		return
	}
	replyType := httpx.BodyInt(c, "reply_type")
	status := httpx.BodyInt(c, "status")
	if replyType != 2 && status == 1 {
		tdb(c).Model(&model.OfficialAccountReply{}).
			Where("reply_type = ? AND id <> ? AND tenant_id = ? AND delete_time IS NULL", replyType, httpx.BodyUint(c, "id"), tid).
			Update("status", 0)
	}
	q := tdb(c).Model(&model.OfficialAccountReply{}).Where("id = ? AND tenant_id = ?", httpx.BodyUint(c, "id"), tid)
	q.Updates(map[string]any{
		"name": httpx.BodyStr(c, "name"), "keyword": httpx.BodyStr(c, "keyword"),
		"reply_type": replyType, "matching_type": httpx.BodyInt(c, "matching_type"),
		"content_type": httpx.BodyInt(c, "content_type"), "content": httpx.BodyStr(c, "content"),
		"status": status, "sort": httpx.BodyInt(c, "sort"), "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "操作成功")
}

func oaReplyByID(c *gin.Context, id uint) (model.OfficialAccountReply, bool) {
	var row model.OfficialAccountReply
	tid, ok := requireTenant(c)
	if !ok || id == 0 {
		return row, false
	}
	if tdb(c).Where("id = ? AND tenant_id = ? AND delete_time IS NULL", id, tid).First(&row).Error != nil || row.ID == 0 {
		return row, false
	}
	return row, true
}

func OAReplyDelete(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if msg := util.OAReplyIDCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	tid, ok := requireTenant(c)
	if !ok {
		response.SuccessNotice(c, "操作成功")
		return
	}
	tdb(c).Unscoped().Where("id = ? AND tenant_id = ?", httpx.BodyUint(c, "id"), tid).Delete(&model.OfficialAccountReply{})
	response.SuccessNotice(c, "操作成功")
}

func OAReplyDetail(c *gin.Context) {
	if msg := util.OAReplyIDCheck(httpx.Query(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	row, ok := oaReplyByID(c, httpx.QueryUint(c, "id"))
	if !ok {
		response.Data(c, []any{})
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
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if msg := util.OAReplyIDCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	row, ok := oaReplyByID(c, httpx.BodyUint(c, "id"))
	if !ok {
		response.SuccessNotice(c, "操作成功")
		return
	}
	status := 0
	if row.Status == 0 {
		status = 1
	}
	// PHP OfficialAccountReplyLogic::status only flips this row.
	tid := tenantDB(c)
	tdb(c).Model(&model.OfficialAccountReply{}).Where("id = ? AND tenant_id = ?", row.ID, tid).Updates(map[string]any{
		"status": status, "update_time": util.NowUnix(),
	})
	response.SuccessNotice(c, "操作成功")
}

func OAReplySort(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	if msg := util.OAReplyIDCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	if msg := util.OAReplySortCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	tid, ok := requireTenant(c)
	if !ok {
		response.SuccessNotice(c, "操作成功")
		return
	}
	sort := httpx.BodyInt(c, "new_sort")
	tdb(c).Model(&model.OfficialAccountReply{}).Where("id = ? AND tenant_id = ?", httpx.BodyUint(c, "id"), tid).
		Updates(map[string]any{"sort": sort, "update_time": util.NowUnix()})
	response.SuccessNotice(c, "操作成功")
}

func OAMenuDetail(c *gin.Context) {
	data := cfgsvc.Get(c, "oa_setting", "menu", []any{})
	response.Data(c, util.CoerceOAMenuHasMenu(data))
}

func OAMenuSave(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	menu := httpx.List(c)
	if menu == nil {
		if v := httpx.BodyAny(c, "menu"); v != nil {
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
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	menu := httpx.List(c)
	if menu == nil {
		if v := httpx.BodyAny(c, "menu"); v != nil {
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
	appID, secret, _ := wechat.OAConfig(c)
	if appID == "" || secret == "" {
		response.Fail(c, "请先设置公众号配置")
		return
	}
	if err := wechat.PublishMenu(appID, secret, menu); err != nil {
		response.Fail(c, err.Error())
		return
	}
	cfgsvc.Set(c, "oa_setting", "menu", menu)
	response.SuccessNotice(c, "保存并发布成功")
}

func TenantNoticeLists(c *gin.Context) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := tdb(c).Model(&model.TenantNoticeSetting{})
	tid := tenantDB(c)
	if tid == 0 {
		response.Lists(c, []map[string]any{}, 0, q.PageNo, q.PageSize, nil)
		return
	}
	db = db.Where("tenant_id = ?", tid)
	if lists.Param(q, "recipient") != "" {
		db = db.Where("recipient = ?", lists.ParamInt(q, "recipient"))
	}
	if lists.Param(q, "type") != "" {
		db = db.Where("type = ?", lists.ParamInt(q, "type"))
	}
	var count int64
	db.Count(&count)
	var rows []model.TenantNoticeSetting
	// PHP TenantNoticeSettingLists::lists() uses select() with no limit.
	db.Order("id asc").Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		sms := util.DecodeJSON(r.SmsNotice)
		out = append(out, map[string]any{
			"id": r.ID, "scene_name": r.SceneName, "sms_notice": sms, "type": r.Type,
			"sms_status_desc": util.SMSStatusDesc(r.SmsNotice), "type_desc": util.NoticeTypeDesc(r.Type),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func tenantNoticeByID(c *gin.Context, id uint) (model.TenantNoticeSetting, bool) {
	var r model.TenantNoticeSetting
	// PHP TenantNoticeSetting::findOrEmpty is scoped to the current tenant.
	// Template rows (tenant_id=0) must stay invisible. Fail closed if the
	// request has no tenant, instead of leaking platform templates.
	tid := tenantDB(c)
	if id == 0 || tid == 0 {
		return r, false
	}
	if tdb(c).Where("id = ? AND tenant_id = ?", id, tid).First(&r).Error != nil || r.ID == 0 || r.TenantID != tid {
		return r, false
	}
	return r, true
}

func TenantNoticeDetail(c *gin.Context) {
	if !httpx.QueryIDPresent(c) {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.QueryUint(c, "id")
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
	if !response.RequirePOST(c) {
		return
	}
	if !guardTenantWrite(c) {
		return
	}
	id := httpx.BodyUint(c, "id")
	_, exists := tenantNoticeByID(c, id)
	updates, err := biz.ApplyNoticeSet(exists, id, httpx.BodyAny(c, "template"))
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	q := tdb(c).Model(&model.TenantNoticeSetting{}).Where("id = ?", id)
	if tid := tenantDB(c); tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	} else {
		response.Fail(c, "通知配置不存在")
		return
	}
	q.Updates(updates)
	response.Success(c, "设置成功", nil)
}

func SettingUserGetConfig(c *gin.Context)   { platformapi.UserGetConfig(c) }
func SettingUserSetConfig(c *gin.Context)   { platformapi.UserSetConfig(c) }
func SettingUserGetRegister(c *gin.Context) { platformapi.UserGetRegisterConfig(c) }
func SettingUserSetRegister(c *gin.Context) { platformapi.UserSetRegisterConfig(c) }
func SettingTransactionGet(c *gin.Context)  { platformapi.TransactionGet(c) }
func SettingTransactionSet(c *gin.Context)  { platformapi.TransactionSet(c) }
func SettingCustomerGet(c *gin.Context)     { platformapi.CustomerGet(c) }
func SettingCustomerSet(c *gin.Context)     { platformapi.CustomerSet(c) }
