package cron

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
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
	if !runOnceMu.TryLock() {
		return
	}
	defer runOnceMu.Unlock()
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
		if !claimDueJob(item, now) {
			continue
		}
		start := time.Now()
		errMsg := runCommand(item)
		// PHP Crontab::start finally writes last_time = time() after the job.
		bootstrap.DB.Model(&item).Updates(crontabFinishUpdates(item, start, errMsg))
	}
}

// crontabFinishUpdates mirrors PHP Crontab::start finally block.
func crontabFinishUpdates(item model.Crontab, start time.Time, errMsg string) map[string]any {
	elapsed := time.Since(start).Seconds()
	maxTime := elapsed
	if prev := util.ToFloat(item.MaxTime); prev > maxTime {
		maxTime = prev
	}
	updates := map[string]any{
		"last_time": util.NowUnix(),
		"time":      fmt.Sprintf("%.2f", elapsed),
		"max_time":  fmt.Sprintf("%.2f", maxTime),
		"error":     "",
	}
	if errMsg != "" {
		updates["error"] = errMsg
		updates["status"] = 3
	}
	return updates
}

func due(item model.Crontab, now int64) bool {
	return biz.CronDue(item.Expression, item.LastTime, now)
}

// claimDueJob marks a due row as started so a concurrent worker or HTTP
// /crontab hit cannot run the same job in the same tick.
func claimDueJob(item model.Crontab, now int64) bool {
	if bootstrap.DB == nil || item.ID == 0 {
		return false
	}
	q := bootstrap.DB.Model(&model.Crontab{}).Where("id = ? AND status = 1 AND delete_time IS NULL", item.ID)
	if item.LastTime != nil {
		q = q.Where("last_time = ?", *item.LastTime)
	} else {
		q = q.Where("last_time IS NULL")
	}
	res := q.Update("last_time", now)
	return res.Error == nil && res.RowsAffected == 1
}

// CommandFunc is a php-think compatible job. Return "" on success or an
// error string (PHP Crontab writes that into la_dev_crontab.error).
type CommandFunc func(args []string) string

var (
	commandMu sync.RWMutex
	commands  = map[string]CommandFunc{}
	runOnceMu sync.Mutex
)

func init() {
	registerBuiltins()
}

func registerBuiltins() {
	Register("cache", func([]string) string { return flushCache() })
	Register("clear", runClear)
	Register("session", func([]string) string {
		expireSessions()
		return ""
	})
	Register("query_refund", func([]string) string { return queryRefund() })
	Register("cancel_unpaid_orders", func([]string) string { return cancelUnpaidOrders() })
	Register("verification_orders", func([]string) string { return verificationOrders() })
	Register("version", runVersion)
	Register("optimize:schema", runOptimizeSchema)
	Register("help", runHelp)
	Register("list", runList)
	Register("vendor:publish", runVendorPublish)
	Register("service:discover", runServiceDiscover)
	Register("build", runBuild)
	Register("make:controller", makeRunner("controller"))
	Register("make:model", makeRunner("model"))
	Register("make:validate", makeRunner("validate"))
	Register("make:middleware", makeRunner("middleware"))
	Register("make:event", makeRunner("event"))
	Register("make:listener", makeRunner("listener"))
	Register("make:subscribe", makeRunner("subscribe"))
	Register("make:service", makeRunner("service"))
	Register("make:command", makeRunner("command"))
}

// Register adds a crontab / `think` command. Unknown warehouse commands stay
// undefined unless a Go hook is registered here — never exec `php think`.
func Register(name string, fn CommandFunc) {
	if fn == nil {
		return
	}
	key := normalizeCommand(name)
	if key == "" {
		key = "cache"
	}
	commandMu.Lock()
	commands[key] = fn
	commandMu.Unlock()
}

// Unregister removes a previously registered command (tests / custom hooks).
func Unregister(name string) {
	key := normalizeCommand(name)
	if key == "" {
		key = "cache"
	}
	commandMu.Lock()
	delete(commands, key)
	commandMu.Unlock()
}

// CommandNames lists registered think/crontab commands, sorted.
func CommandNames() []string {
	commandMu.RLock()
	out := make([]string, 0, len(commands))
	for k := range commands {
		out = append(out, k)
	}
	commandMu.RUnlock()
	sort.Strings(out)
	return out
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
	if cmd == "crontab" {
		// A la_dev_crontab row must not recurse into RunOnce.
		return fmt.Sprintf("未定义的定时任务命令: %s", item.Command)
	}
	if cmd == "" {
		cmd = "cache"
	}
	commandMu.RLock()
	fn := commands[cmd]
	commandMu.RUnlock()
	if fn == nil {
		if msg, ok := runThinkHook(cmd, args); ok {
			return msg
		}
		log.Printf("crontab skip unsupported command %s", cmd)
		return fmt.Sprintf("未定义的定时任务命令: %s", item.Command)
	}
	return fn(args)
}

func normalizeCommand(raw string) string {
	cmd := strings.ToLower(strings.TrimSpace(raw))
	cmd = strings.ReplaceAll(cmd, "\\", "/")
	switch {
	case strings.Contains(cmd, "query_refund") || strings.Contains(cmd, "queryrefund"):
		return "query_refund"
	case strings.Contains(cmd, "cancel_unpaid") || strings.Contains(cmd, "cancelunpaid"):
		return "cancel_unpaid_orders"
	case strings.Contains(cmd, "verification_order") || strings.Contains(cmd, "verificationorder"):
		return "verification_orders"
	case strings.Contains(cmd, "session") || strings.Contains(cmd, "token"):
		return "session"
	case cmd == "clear" || strings.HasSuffix(cmd, "/clear"):
		return "clear"
	case cmd == "crontab":
		return "crontab"
	case cmd == "version" || strings.HasSuffix(cmd, "/version"):
		return "version"
	case strings.Contains(cmd, "optimize:schema") || strings.Contains(cmd, "optimizeschema"):
		return "optimize:schema"
	case strings.Contains(cmd, "optimize:route") || strings.Contains(cmd, "optimizeroute"):
		return "optimize:route"
	case strings.Contains(cmd, "route:list") || strings.Contains(cmd, "routelist"):
		return "route:list"
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

type clearOpts struct {
	cache, log, rmdir, expire bool
	path, app                 string
}

func parseClearArgs(args []string) clearOpts {
	var o clearOpts
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--cache" || a == "-c":
			o.cache = true
		case a == "--log" || a == "-l":
			o.log = true
		case a == "--dir" || a == "-r":
			o.rmdir = true
		case a == "--expire" || a == "-e":
			o.expire = true
		case a == "--path" || a == "-d":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				o.path = args[i]
			}
		case strings.HasPrefix(a, "--path="):
			o.path = strings.TrimPrefix(a, "--path=")
		case strings.HasPrefix(a, "-d="):
			o.path = strings.TrimPrefix(a, "-d=")
		default:
			// PHP think-multi-app: optional app argument → runtime/<app>
			if !strings.HasPrefix(a, "-") && o.app == "" {
				o.app = a
			}
		}
	}
	return o
}

func runClear(args []string) string {
	o := parseClearArgs(args)
	expireOnly := o.expire && o.cache
	var msg string
	switch {
	case o.log && !o.cache:
		msg = clearRuntimeNamed("log", o.rmdir, false)
	case o.cache:
		msg = clearRuntimeNamed("cache", o.rmdir, expireOnly)
	case o.path != "":
		msg = clearCustomPath(o.path, o.rmdir)
	case o.app != "":
		msg = clearRuntimeNamed(o.app, o.rmdir, false)
	default:
		// PHP Clear (multi-app + framework) only wipes runtime files.
		// Application/Redis cache stays; use `think cache` for that.
		msg = clearRuntimeDir(filepath.Join(runtimeRoot(), "runtime"), o.rmdir, false)
	}
	if msg != "" {
		return msg
	}
	fmt.Println("Clear Successed")
	return ""
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
	return clearRuntimeDir(filepath.Join(root, "runtime"), false, false)
}

func clearRuntimeNamed(name string, rmdir, expireOnly bool) string {
	root := runtimeRoot()
	if root == "" {
		return ""
	}
	return clearRuntimeDir(filepath.Join(root, "runtime", name), rmdir, expireOnly)
}

func clearCustomPath(p string, rmdir bool) string {
	root := runtimeRoot()
	if root == "" || strings.TrimSpace(p) == "" {
		return ""
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	target := p
	if !filepath.IsAbs(target) {
		target = filepath.Join(absRoot, p)
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "清理路径不合法"
	}
	return clearRuntimeDir(abs, rmdir, false)
}

func clearRuntimeDir(dir string, rmdir, expireOnly bool) string {
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return ""
	}
	var dirs []string
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
			dirs = append(dirs, path)
			return nil
		}
		if expireOnly && !cacheHasExpired(path) {
			return nil
		}
		_ = os.Remove(path)
		return nil
	})
	if rmdir {
		for i := len(dirs) - 1; i >= 0; i-- {
			_ = os.Remove(dirs[i])
		}
	}
	return ""
}

// cacheHasExpired mirrors ThinkPHP Clear::cacheHasExpired:
// expire = (int) substr($content, 8, 12); expired if expire != 0 && time() - expire > filemtime.
func cacheHasExpired(filename string) bool {
	b, err := os.ReadFile(filename)
	if err != nil || len(b) < 20 {
		return false
	}
	expire, err := strconv.Atoi(strings.TrimSpace(string(b[8:20])))
	if err != nil || expire == 0 {
		return false
	}
	st, err := os.Stat(filename)
	if err != nil {
		return false
	}
	return time.Now().Unix()-int64(expire) > st.ModTime().Unix()
}

func queryRefund() string {
	if bootstrap.DB == nil {
		return ""
	}
	for _, lg := range listRefundingLogs() {
		applyRefundQuery(lg)
	}
	return ""
}

// listRefundingLogs mirrors PHP QueryRefund::execute: INNER JOIN refund_record
// and only refund_status = REFUND_ING (0) logs.
func listRefundingLogs() []model.RefundLog {
	var logs []model.RefundLog
	if bootstrap.DB == nil {
		return logs
	}
	logT := model.RefundLog{}.TableName()
	recT := model.RefundRecord{}.TableName()
	bootstrap.DB.Table(logT+" AS l").
		Select("l.*").
		Joins("JOIN "+recT+" AS r ON r.id = l.record_id").
		Where("l.refund_status = ?", 0).
		Find(&logs)
	return logs
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
	now := util.NowUnix()
	lq.Updates(map[string]any{"refund_status": 1, "update_time": now})
	rq.Updates(map[string]any{"refund_status": 1, "update_time": now})
	if rec.OrderType == "recharge" && rec.OrderID > 0 && refundTID != "" {
		oq := bootstrap.DB.Model(&model.RechargeOrder{}).Where("id = ?", rec.OrderID)
		if rec.TenantID > 0 {
			oq = oq.Where("tenant_id = ?", rec.TenantID)
		}
		oq.Updates(map[string]any{"refund_transaction_id": refundTID, "update_time": now})
	}
}

func updateRefundMsg(lg model.RefundLog, msg string) {
	q := bootstrap.DB.Model(&model.RefundLog{}).Where("id = ?", lg.ID)
	if lg.TenantID > 0 {
		q = q.Where("tenant_id = ?", lg.TenantID)
	}
	q.Updates(map[string]any{"refund_msg": msg, "update_time": util.NowUnix()})
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
	q.Updates(util.SoftDeleteFields(now))
}

// verificationOrders is the worker behind transaction.verification_orders.
// Stock likeadmin has no pickup/verify order table (only recharge); the
// command is registered so crontab rows and think CLI succeed without PHP.
func verificationOrders() string {
	if bootstrap.DB == nil {
		return ""
	}
	now := util.NowUnix()
	platOn, platHours := txnVerifySettings(0, 1, 24)
	var tenants []model.Tenant
	bootstrap.DB.Where("delete_time IS NULL").Find(&tenants)
	if len(tenants) == 0 {
		verifyOrdersForTenant(0, platOn, platHours, now)
		return ""
	}
	for _, t := range tenants {
		on, hours := txnVerifySettings(t.ID, platOn, platHours)
		verifyOrdersForTenant(t.ID, on, hours, now)
	}
	return ""
}

func txnVerifySettings(tenantID uint, defOn, defHours int) (enabled, hours int) {
	enabled, hours = defOn, defHours
	type kv struct {
		Name  string `gorm:"column:name"`
		Value string `gorm:"column:value"`
	}
	var rows []kv
	if tenantID > 0 {
		db := tenantdb.ForTenant(tenantID)
		if db == nil {
			return enabled, hours
		}
		db.Model(&model.TenantConfig{}).Where("tenant_id = ? AND type = ?", tenantID, "transaction").
			Select("name, value").Scan(&rows)
	} else {
		bootstrap.DB.Model(&model.ConfigRow{}).Where("type = ?", "transaction").
			Select("name, value").Scan(&rows)
	}
	for _, r := range rows {
		switch r.Name {
		case "verification_orders":
			enabled = util.ToInt(r.Value)
		case "verification_orders_times":
			if n := util.ToInt(r.Value); n > 0 {
				hours = n
			}
		}
	}
	return enabled, hours
}

func verifyOrdersForTenant(tenantID uint, enabled, hours int, now int64) {
	if enabled != 1 || hours <= 0 {
		return
	}
	cutoff := now - int64(hours)*3600
	verifyTableOrders(bootstrap.DB, tenantID, cutoff, now, "la_recharge_order")
}

func verifyTableOrders(db *gorm.DB, tenantID uint, cutoff, now int64, table string) {
	if db == nil || table == "" {
		return
	}
	if !tableHasColumn(db, table, "verify_status") && !tableHasColumn(db, table, "is_verify") {
		return
	}
	col := "verify_status"
	if !tableHasColumn(db, table, col) {
		col = "is_verify"
	}
	q := db.Table(table).Where(col+" = 0 AND delete_time IS NULL AND create_time > 0 AND create_time < ?", cutoff)
	if tenantID > 0 && tableHasColumn(db, table, "tenant_id") {
		q = q.Where("tenant_id = ?", tenantID)
	}
	fields := map[string]any{col: 1, "update_time": now}
	if tableHasColumn(db, table, "verify_time") {
		fields["verify_time"] = now
	}
	_ = q.Updates(fields).Error
}

func tableHasColumn(db *gorm.DB, table, col string) bool {
	if db == nil || table == "" || col == "" {
		return false
	}
	var n int64
	if db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, col).Scan(&n).Error != nil {
		return false
	}
	return n > 0
}

// EnsureNativeJobs inserts the Go-only system jobs a PHP install never shipped,
// so cmd/crontab still queries refunds, cancels stale unpaid orders, and runs
// verification_orders after php-fpm is stopped.
func EnsureNativeJobs() {
	if bootstrap.DB == nil {
		return
	}
	now := util.NowUnix()
	last := now - 90
	for _, job := range []model.Crontab{
		{Name: "查询退款状态", Command: "query_refund", Remark: "查询微信/支付宝退款结果"},
		{Name: "取消超时未支付订单", Command: "cancel_unpaid_orders", Remark: "按交易设置取消超时未支付充值单"},
		{Name: "自动核销订单", Command: "verification_orders", Remark: "按交易设置核销超时未核销订单"},
	} {
		var rows []model.Crontab
		if err := nativeSystemJobs(job.Command).Order("id asc").Find(&rows).Error; err != nil {
			log.Printf("crontab native job lookup %s: %v", job.Command, err)
			continue
		}
		if len(rows) == 0 {
			job.Type = 1
			job.System = 1
			job.Status = 1
			job.Expression = "* * * * *"
			job.LastTime = &last
			job.Time = "0"
			job.MaxTime = "0"
			job.CreateTime = now
			job.UpdateTime = util.UnixPtr(now)
			if err := bootstrap.DB.Create(&job).Error; err != nil {
				log.Printf("crontab native job insert %s: %v", job.Command, err)
			}
			continue
		}
		for _, extra := range rows[1:] {
			if err := bootstrap.DB.Model(&extra).Update("delete_time", now).Error; err != nil {
				log.Printf("crontab native job dedupe %s id=%d: %v", job.Command, extra.ID, err)
			}
		}
	}
}

func nativeSystemJobs(command string) *gorm.DB {
	return bootstrap.DB.Model(&model.Crontab{}).Where("command = ? AND `system` = 1 AND delete_time IS NULL", command)
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
