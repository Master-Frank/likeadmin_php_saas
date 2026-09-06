package cron

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"likeadmin/backend/internal/biz"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	paycfg "likeadmin/backend/internal/pay"
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
		applyRefundQuery(lg)
	}
	return ""
}

func applyRefundQuery(lg model.RefundLog) {
	var rec model.RefundRecord
	if bootstrap.DB.First(&rec, lg.RecordID).Error != nil {
		return
	}
	if rec.OrderType != "recharge" {
		return
	}
	var order model.RechargeOrder
	if bootstrap.DB.First(&order, rec.OrderID).Error != nil {
		return
	}
	if order.PayWay != 2 {
		return
	}
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
