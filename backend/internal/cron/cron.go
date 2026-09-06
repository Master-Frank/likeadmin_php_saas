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
	cmd := strings.TrimSpace(item.Command)
	switch {
	case cmd == "" || strings.Contains(cmd, "cache"):
		if bootstrap.RDB != nil {
			if err := bootstrap.RDB.FlushDB(context.Background()).Err(); err != nil {
				return err.Error()
			}
		}
		return ""
	default:
		log.Printf("crontab skip unsupported command %s", cmd)
		return ""
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
