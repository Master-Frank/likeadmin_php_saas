package cron

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
	paycfg "likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
)

func RunOnce() {
	if bootstrap.DB == nil {
		return
	}
	var rows []model.Crontab
	bootstrap.DB.Where("status = 1").Find(&rows)
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

func runCommand(item model.Crontab) string {
	cmd := normalizeCommand(item.Command)
	_ = strings.Fields(strings.TrimSpace(item.Params))
	switch {
	case cmd == "cache" || cmd == "":
		return flushCache()
	case cmd == "clear":
		if msg := flushCache(); msg != "" {
			return msg
		}
		return clearRuntime()
	case cmd == "session":
		expireSessions()
		return ""
	case cmd == "query_refund":
		return queryRefund()
	default:
		if ok, msg := runThinkCommand(item); ok {
			return msg
		}
		log.Printf("crontab skip unsupported command %s", cmd)
		return fmt.Sprintf("未定义的定时任务命令: %s", item.Command)
	}
}

func runThinkCommand(item model.Crontab) (bool, string) {
	root := ""
	if pub := config.C.App.PublicDir; pub != "" {
		root = filepath.Dir(pub)
	}
	if root == "" {
		return false, ""
	}
	if _, err := os.Stat(filepath.Join(root, "think")); err != nil {
		return false, ""
	}
	php, err := exec.LookPath("php")
	if err != nil {
		return false, ""
	}
	args := []string{"think", strings.TrimSpace(item.Command)}
	if p := strings.TrimSpace(item.Params); p != "" {
		args = append(args, strings.Fields(p)...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, php, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err == nil {
		return true, ""
	}
	low := strings.ToLower(text + " " + err.Error())
	if strings.Contains(low, "not defined") || strings.Contains(low, "does not exist") ||
		strings.Contains(low, "not found") || strings.Contains(low, "未定义") {
		return false, ""
	}
	if text == "" {
		text = err.Error()
	}
	return true, text
}

func normalizeCommand(raw string) string {
	cmd := strings.ToLower(strings.TrimSpace(raw))
	cmd = strings.ReplaceAll(cmd, "\\", "/")
	switch {
	case strings.Contains(cmd, "query_refund") || strings.Contains(cmd, "queryrefund"):
		return "query_refund"
	case strings.Contains(cmd, "session") || strings.Contains(cmd, "token"):
		return "session"
	case cmd == "clear" || strings.HasSuffix(cmd, "/clear"):
		return "clear"
	case cmd == "" || strings.Contains(cmd, "cache") || cmd == "crontab":
		return "cache"
	default:
		return cmd
	}
}

func flushCache() string {
	if bootstrap.RDB != nil {
		if err := bootstrap.RDB.FlushDB(context.Background()).Err(); err != nil {
			return err.Error()
		}
	}
	return ""
}

func expireSessions() {
	now := util.NowUnix()
	if bootstrap.DB == nil {
		return
	}
	bootstrap.DB.Where("expire_time < ?", now).Delete(&model.AdminSession{})
	bootstrap.DB.Where("expire_time < ?", now).Delete(&model.TenantAdminSession{})
	bootstrap.DB.Where("expire_time < ?", now).Delete(&model.UserSession{})
	var tenants []model.Tenant
	bootstrap.DB.Where("tactics = 1 AND delete_time IS NULL AND sn <> ''").Find(&tenants)
	for _, t := range tenants {
		db := tenantdb.UseSN(t.SN)
		if db == nil {
			continue
		}
		db.Where("expire_time < ?", now).Delete(&model.TenantAdminSession{})
		db.Where("expire_time < ?", now).Delete(&model.UserSession{})
	}
}

func clearRuntime() string {
	root := ""
	if pub := config.C.App.PublicDir; pub != "" {
		root = filepath.Dir(pub)
	}
	if root == "" {
		return ""
	}
	dir := filepath.Join(root, "runtime")
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return ""
	}
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return nil
		}
		name := d.Name()
		if name == "." || name == ".." {
			return nil
		}
		if strings.HasPrefix(name, "curd-") && strings.HasSuffix(name, ".zip") {
			return nil
		}
		if d.IsDir() && (name == "cache" || name == "temp" || name == "log") {
			_ = os.RemoveAll(path)
			_ = os.MkdirAll(path, 0755)
			return fs.SkipDir
		}
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
			updateRefundSuccess(lg.ID, rec.ID)
			return
		}
		updateRefundMsg(lg.ID, "微信:"+msg)
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
			updateRefundSuccess(lg.ID, rec.ID)
			return
		}
		updateRefundMsg(lg.ID, "支付宝:"+msg)
	}
}

func updateRefundSuccess(logID, recordID uint) {
	bootstrap.DB.Model(&model.RefundLog{}).Where("id = ?", logID).Update("refund_status", 1)
	bootstrap.DB.Model(&model.RefundRecord{}).Where("id = ?", recordID).Update("refund_status", 1)
}

func updateRefundMsg(logID uint, msg string) {
	bootstrap.DB.Model(&model.RefundLog{}).Where("id = ?", logID).Update("refund_msg", msg)
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
