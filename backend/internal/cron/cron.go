package cron

import (
	"context"
	"log"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
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
		if !due(item, now) {
			continue
		}
		start := time.Now()
		errMsg := runCommand(item)
		updates := map[string]any{
			"last_time": now,
			"time":      time.Since(start).String(),
			"error":     errMsg,
		}
		if errMsg != "" {
			updates["status"] = 3
		}
		bootstrap.DB.Model(&item).Updates(updates)
	}
}

func due(item model.Crontab, now int64) bool {
	if item.LastTime == nil {
		return true
	}
	interval := parseInterval(item.Expression)
	return now-*item.LastTime >= interval
}

func parseInterval(expr string) int64 {
	expr = strings.TrimSpace(expr)
	if expr == "" || expr == "* * * * *" {
		return 60
	}
	parts := strings.Fields(expr)
	if len(parts) >= 1 && strings.HasPrefix(parts[0], "*/") {
		n := util.ParseInt(strings.TrimPrefix(parts[0], "*/"))
		if n > 0 {
			return int64(n) * 60
		}
	}
	return 60
}

func runCommand(item model.Crontab) string {
	cmd := strings.ToLower(strings.TrimSpace(item.Command))
	switch {
	case cmd == "" || strings.Contains(cmd, "cache"):
		if bootstrap.RDB != nil {
			if err := bootstrap.RDB.FlushDB(context.Background()).Err(); err != nil {
				return err.Error()
			}
		}
		return ""
	case strings.Contains(cmd, "session") || strings.Contains(cmd, "token"):
		now := util.NowUnix()
		bootstrap.DB.Where("expire_time < ?", now).Delete(&model.AdminSession{})
		bootstrap.DB.Where("expire_time < ?", now).Delete(&model.TenantAdminSession{})
		bootstrap.DB.Where("expire_time < ?", now).Delete(&model.UserSession{})
		return ""
	case strings.Contains(cmd, "query_refund") || strings.Contains(cmd, "refund"):
		return queryRefund()
	default:
		log.Printf("crontab skip unsupported command %s", cmd)
		return ""
	}
}

func queryRefund() string {
	if bootstrap.DB == nil {
		return ""
	}
	var logs []model.RefundLog
	bootstrap.DB.Where("refund_status = 0").Find(&logs)
	for _, lg := range logs {
		var rec model.RefundRecord
		if bootstrap.DB.First(&rec, lg.RecordID).Error != nil {
			continue
		}
		if rec.OrderType != "recharge" {
			continue
		}
		var order model.RechargeOrder
		if bootstrap.DB.First(&order, rec.OrderID).Error != nil {
			continue
		}
		if order.PayWay != 2 && order.PayWay != 3 {
			continue
		}
		// Without live gateway credentials, leave in-progress records untouched.
	}
	return ""
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
