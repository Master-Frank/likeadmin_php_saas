package cron

import (
	"fmt"
	"log"
	"strings"
	"time"

	"io/fs"
	"os"
	"path/filepath"

	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
	paycfg "likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
)

func RunOnce() {
	if bootstrap.DB == nil {
		return
	}
	tenantdb.Register(bootstrap.DB)
	EnsureNativeJobs()
	var rows []model.Crontab
	bootstrap.DB.Where("status = 1 AND delete_time IS NULL").Find(&rows)
	now := util.NowUnix()
	for _, item := range rows {
		if item.LastTime == nil || *item.LastTime <= 0 {
			if e, err := biz.ParseCron(item.Expression); err == nil {
				if next := e.Next(time.Unix(now, 0)); !next.IsZero() {
					ts := next.Unix()
					bootstrap.DB.Model(&item).Update("last_time", ts)
				}
			}
			continue
		}
		if !due(item, now) {
			continue
		}
		start := time.Now()
		errMsg := runCommand(item)
		elapsed := time.Since(start).Seconds()
		maxTime := elapsed
		if prev := util.ToFloat(item.MaxTime); prev > maxTime {
			maxTime = prev
		}
		updates := map[string]any{
			"last_time": now,
			"time":      fmt.Sprintf("%.2f", elapsed),
			"max_time":  fmt.Sprintf("%.2f", maxTime),
			"error":     errMsg,
		}
		if errMsg != "" {
			updates["status"] = 3
		} else {
			updates["error"] = ""
		}
		bootstrap.DB.Model(&item).Updates(updates)
	}
}

func due(item model.Crontab, now int64) bool {
	return biz.CronDue(item.Expression, item.LastTime, now)
}

// RunNamed runs a php-think compatible command (cache/clear/session/query_refund/...).
// `think crontab` is the scheduler once-shot (PHP Crontab::execute), not cache flush.
func RunNamed(command string, params ...string) string {
	if normalizeCommand(command) == "crontab" {
		RunOnce()
		return ""
	}
	return runCommand(model.Crontab{Command: command, Params: strings.Join(params, " ")})
}

func runCommand(item model.Crontab) string {
	cmd := normalizeCommand(item.Command)
	args := strings.Fields(strings.TrimSpace(item.Params))
	switch {
	case cmd == "cache" || cmd == "":
		return flushCache()
	case cmd == "clear":
		return runClear(args)
	case cmd == "session":
		expireSessions()
		return ""
	case cmd == "query_refund":
		return queryRefund()
	case cmd == "cancel_unpaid_orders":
		return cancelUnpaidOrders()
	case cmd == "crontab":
		// A la_dev_crontab row must not recurse into RunOnce.
		return fmt.Sprintf("未定义的定时任务命令: %s", item.Command)
	default:
		log.Printf("crontab skip unsupported command %s", cmd)
		return fmt.Sprintf("未定义的定时任务命令: %s", item.Command)
	}
}

func normalizeCommand(raw string) string {
	cmd := strings.ToLower(strings.TrimSpace(raw))
	cmd = strings.ReplaceAll(cmd, "\\", "/")
	switch {
	case strings.Contains(cmd, "query_refund") || strings.Contains(cmd, "queryrefund"):
		return "query_refund"
	case strings.Contains(cmd, "cancel_unpaid") || strings.Contains(cmd, "cancelunpaid"):
		return "cancel_unpaid_orders"
	case strings.Contains(cmd, "session") || strings.Contains(cmd, "token"):
		return "session"
	case cmd == "clear" || strings.HasSuffix(cmd, "/clear"):
		return "clear"
	case cmd == "crontab":
		return "crontab"
	case cmd == "" || strings.Contains(cmd, "cache"):
		return "cache"
	default:
		return cmd
	}
}

func flushCache() string {
	cache.Flush()
	return ""
}

func expireSessions() {
	now := util.NowUnix()
	if bootstrap.DB == nil {
		return
	}
	purgeExpiredSessions(bootstrap.DB, now, true)
	var tenants []model.Tenant
	bootstrap.DB.Where("tactics = 1 AND delete_time IS NULL AND sn <> ''").Find(&tenants)
	for _, t := range tenants {
		db := tenantdb.UseSN(t.SN)
		if db == nil {
			continue
		}
		purgeExpiredSessions(db, now, false)
	}
}

func purgeExpiredSessions(db *gorm.DB, now int64, includePlatform bool) {
	if db == nil {
		return
	}
	if includePlatform {
		var admins []model.AdminSession
		db.Where("expire_time < ?", now).Find(&admins)
		dropSessionTokens(admins, nil, nil)
		db.Where("expire_time < ?", now).Delete(&model.AdminSession{})
	}
	var tenants []model.TenantAdminSession
	db.Where("expire_time < ?", now).Find(&tenants)
	var users []model.UserSession
	db.Where("expire_time < ?", now).Find(&users)
	dropSessionTokens(nil, tenants, users)
	db.Where("expire_time < ?", now).Delete(&model.TenantAdminSession{})
	db.Where("expire_time < ?", now).Delete(&model.UserSession{})
}

func dropSessionTokens(admins []model.AdminSession, tenants []model.TenantAdminSession, users []model.UserSession) {
	for _, s := range admins {
		if s.Token != "" {
			cache.DeleteAdminInfo(s.Token)
		}
	}
	for _, s := range tenants {
		if s.Token != "" {
			cache.DeleteTenantAdminInfo(s.Token)
		}
	}
	for _, s := range users {
		if s.Token != "" {
			cache.DeleteUserInfo(s.Token)
		}
	}
}

func runClear(args []string) string {
	cacheOnly, logOnly := false, false
	for _, a := range args {
		switch a {
		case "--cache", "-c":
			cacheOnly = true
		case "--log", "-l":
			logOnly = true
		}
	}
	if logOnly && !cacheOnly {
		return clearRuntimeNamed("log")
	}
	if cacheOnly {
		return clearRuntimeNamed("cache")
	}
	if msg := flushCache(); msg != "" {
		return msg
	}
	return clearRuntime()
}

func runtimeRoot() string {
	if pub := config.C.App.PublicDir; pub != "" {
		return filepath.Dir(pub)
	}
	return ""
}

func clearRuntime() string {
	root := runtimeRoot()
	if root == "" {
		return ""
	}
	return clearRuntimeDir(filepath.Join(root, "runtime"))
}

func clearRuntimeNamed(name string) string {
	root := runtimeRoot()
	if root == "" {
		return ""
	}
	return clearRuntimeDir(filepath.Join(root, "runtime", name))
}

func clearRuntimeDir(dir string) string {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return ""
	}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}
		name := d.Name()
		if name == "." || name == ".." || name == ".gitignore" {
			return nil
		}
		if name == ".git" || name == "node_modules" || name == "vendor" {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, "curd-") && strings.HasSuffix(name, ".zip") {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		_ = os.Remove(path)
		return nil
	})
	return ""
}

func queryRefund() string {
	if bootstrap.DB == nil {
		return ""
	}
	var logs []model.RefundLog
	bootstrap.DB.Where("refund_status = 0").Find(&logs)
	for _, lg := range logs {
		applyRefundQuery(lg)
	}
	return ""
}

func applyRefundQuery(lg model.RefundLog) {
	var rec model.RefundRecord
	rq := bootstrap.DB.Where("id = ?", lg.RecordID)
	if lg.TenantID > 0 {
		rq = rq.Where("tenant_id = ?", lg.TenantID)
	}
	if rq.First(&rec).Error != nil {
		return
	}
	if rec.OrderType != "recharge" {
		return
	}
	var order model.RechargeOrder
	oq := bootstrap.DB.Where("id = ?", rec.OrderID)
	if rec.TenantID > 0 {
		oq = oq.Where("tenant_id = ?", rec.TenantID)
	}
	if oq.First(&order).Error != nil {
		return
	}
	switch order.PayWay {
	case paycfg.WayWechat:
		cfg := paycfg.WechatCfgByTenant(order.TenantID)
		if cfg.MchID == "" || cfg.APIClientKey == "" {
			return
		}
		result, err := paycfg.WechatQueryRefund(cfg, lg.SN)
		if err != nil || result == nil {
			return
		}
		ok, msg, known := paycfg.ParseWechatRefundQuery(result)
		if !known {
			return
		}
		if ok {
			updateRefundSuccess(lg, rec, paycfg.RefundQueryTradeNo(result))
			return
		}
		updateRefundMsg(lg, "微信:"+msg)
	case paycfg.WayAli:
		result, err := paycfg.AliQueryRefundByTenant(order.TenantID, order.SN, lg.SN)
		if err != nil || result == nil {
			return
		}
		ok, msg, known := paycfg.ParseAliRefundQuery(result)
		if !known {
			return
		}
		if ok {
			updateRefundSuccess(lg, rec, paycfg.RefundQueryTradeNo(result))
			return
		}
		updateRefundMsg(lg, "支付宝:"+msg)
	}
}

func updateRefundSuccess(lg model.RefundLog, rec model.RefundRecord, refundTID string) {
	lq := bootstrap.DB.Model(&model.RefundLog{}).Where("id = ?", lg.ID)
	rq := bootstrap.DB.Model(&model.RefundRecord{}).Where("id = ?", rec.ID)
	if lg.TenantID > 0 {
		lq = lq.Where("tenant_id = ?", lg.TenantID)
	}
	if rec.TenantID > 0 {
		rq = rq.Where("tenant_id = ?", rec.TenantID)
	}
	lq.Update("refund_status", 1)
	rq.Update("refund_status", 1)
	if rec.OrderType == "recharge" && rec.OrderID > 0 && refundTID != "" {
		oq := bootstrap.DB.Model(&model.RechargeOrder{}).Where("id = ?", rec.OrderID)
		if rec.TenantID > 0 {
			oq = oq.Where("tenant_id = ?", rec.TenantID)
		}
		oq.Update("refund_transaction_id", refundTID)
	}
}

func updateRefundMsg(lg model.RefundLog, msg string) {
	q := bootstrap.DB.Model(&model.RefundLog{}).Where("id = ?", lg.ID)
	if lg.TenantID > 0 {
		q = q.Where("tenant_id = ?", lg.TenantID)
	}
	q.Update("refund_msg", msg)
}

func cancelUnpaidOrders() string {
	if bootstrap.DB == nil {
		return ""
	}
	now := util.NowUnix()
	platOn, platMin := txnCancelSettings(0, 1, 30)
	var tenants []model.Tenant
	bootstrap.DB.Where("delete_time IS NULL").Find(&tenants)
	if len(tenants) == 0 {
		cancelUnpaidForTenant(0, platOn, platMin, now)
		return ""
	}
	for _, t := range tenants {
		on, minutes := txnCancelSettings(t.ID, platOn, platMin)
		cancelUnpaidForTenant(t.ID, on, minutes, now)
	}
	return ""
}

func txnCancelSettings(tenantID uint, defOn, defMin int) (enabled, minutes int) {
	enabled, minutes = defOn, defMin
	type kv struct {
		Name  string `gorm:"column:name"`
		Value string `gorm:"column:value"`
	}
	var rows []kv
	if tenantID > 0 {
		db := tenantdb.ForTenant(tenantID)
		if db == nil {
			return enabled, minutes
		}
		db.Model(&model.TenantConfig{}).Where("tenant_id = ? AND type = ?", tenantID, "transaction").
			Select("name, value").Scan(&rows)
	} else {
		bootstrap.DB.Model(&model.ConfigRow{}).Where("type = ?", "transaction").
			Select("name, value").Scan(&rows)
	}
	for _, r := range rows {
		switch r.Name {
		case "cancel_unpaid_orders":
			enabled = util.ToInt(r.Value)
		case "cancel_unpaid_orders_times":
			if n := util.ToInt(r.Value); n > 0 {
				minutes = n
			}
		}
	}
	return enabled, minutes
}

func cancelUnpaidForTenant(tenantID uint, enabled, minutes int, now int64) {
	if enabled != 1 || minutes <= 0 {
		return
	}
	cutoff := now - int64(minutes)*60
	q := bootstrap.DB.Model(&model.RechargeOrder{}).
		Where("pay_status = 0 AND delete_time IS NULL AND create_time > 0 AND create_time < ?", cutoff)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	q.Update("delete_time", now)
}

// EnsureNativeJobs inserts the Go-only system jobs a PHP install never shipped,
// so cmd/crontab still queries refunds and cancels stale unpaid orders after
// php-fpm is stopped.
func EnsureNativeJobs() {
	if bootstrap.DB == nil {
		return
	}
	now := util.NowUnix()
	last := now - 90
	for _, job := range []model.Crontab{
		{Name: "查询退款状态", Command: "query_refund", Remark: "查询微信/支付宝退款结果"},
		{Name: "取消超时未支付订单", Command: "cancel_unpaid_orders", Remark: "按交易设置取消超时未支付充值单"},
	} {
		var n int64
		bootstrap.DB.Model(&model.Crontab{}).Where("command = ? AND system = 1 AND delete_time IS NULL", job.Command).Count(&n)
		if n > 0 {
			continue
		}
		job.Type = 1
		job.System = 1
		job.Status = 1
		job.Expression = "* * * * *"
		job.LastTime = &last
		job.Time = "0"
		job.MaxTime = "0"
		job.CreateTime = now
		_ = bootstrap.DB.Create(&job).Error
	}
}

func Loop(interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	RunOnce()
	t := time.NewTicker(interval)
	defer t.Stop()
	for range t.C {
		RunOnce()
	}
}
