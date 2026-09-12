package bootstrap

import (
	"errors"
	"strings"
	"time"

	"likeadmin/backend/internal/metrics"

	"gorm.io/gorm"
)

func registerReplicaFailover(db *gorm.DB) {
	if db == nil {
		return
	}
	_ = db.Callback().Query().After("gorm:after_query").Register("likeadmin:replica_retry", replicaRetryAfter)
	_ = db.Callback().Row().Before("gorm:row").Register("likeadmin:replica_row_mode", replicaRowMode)
	_ = db.Callback().Row().After("gorm:row").Register("likeadmin:replica_retry_row", replicaRowRetryAfter)
}

func replicaRetryAfter(db *gorm.DB) {
	if db == nil || db.Error == nil || db.Statement == nil {
		return
	}
	if !usingReplica(db) || !isRetryableDBErr(db.Error) {
		return
	}
	if _, ok := db.InstanceGet("likeadmin:replica_retried"); ok {
		return
	}
	MarkReplicaUnhealthy()
	if DB == nil || DB.Config == nil || db.Statement.SQL.Len() == 0 {
		return
	}
	db.InstanceSet("likeadmin:replica_retried", true)
	dest := db.Statement.Dest
	ctx := db.Statement.Context
	vars := db.Statement.Vars
	sql := db.Statement.SQL.String()
	tx := DB.WithContext(ctx).Session(&gorm.Session{NewDB: true})
	var err error
	if dest != nil {
		err = tx.Raw(sql, vars...).Scan(dest).Error
	} else {
		err = tx.Exec(sql, vars...).Error
	}
	if err == nil {
		db.Error = nil
	}
}

func replicaRowMode(db *gorm.DB) {
	if db == nil {
		return
	}
	if isRows, ok := db.Get("rows"); ok {
		db.InstanceSet("likeadmin:replica_rows", isRows)
	}
}

func replicaRowRetryAfter(db *gorm.DB) {
	if db == nil || db.Error == nil || db.Statement == nil ||
		!usingReplica(db) || !isRetryableDBErr(db.Error) {
		return
	}
	MarkReplicaUnhealthy()
	if DB == nil || DB.Config == nil || db.Statement.SQL.Len() == 0 {
		return
	}
	master := DB.WithContext(db.Statement.Context).Session(&gorm.Session{NewDB: true})
	isRows, _ := db.InstanceGet("likeadmin:replica_rows")
	if many, _ := isRows.(bool); many {
		rows, err := master.Statement.ConnPool.QueryContext(
			db.Statement.Context, db.Statement.SQL.String(), db.Statement.Vars...,
		)
		if err == nil {
			db.Statement.Dest = rows
			db.Error = nil
		}
		return
	}
	db.Statement.Dest = master.Statement.ConnPool.QueryRowContext(
		db.Statement.Context, db.Statement.SQL.String(), db.Statement.Vars...,
	)
	db.Error = nil
}

func usingReplica(db *gorm.DB) bool {
	if db == nil || ReadDB == nil || DB == nil || ReadDB == DB {
		return false
	}
	if db.Config == nil || ReadDB.Config == nil {
		return false
	}
	return db.Config == ReadDB.Config
}

// MarkReplicaUnhealthy forces subsequent Read() calls onto the master until
// the next successful health check.
func MarkReplicaUnhealthy() {
	replicaHealth.mu.Lock()
	replicaHealth.live = false
	replicaHealth.checked = time.Now()
	replicaHealth.mu.Unlock()
	metrics.SetReplicaUp(false)
}

func isRetryableDBErr(err error) bool {
	if err == nil || errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, p := range []string{
		"invalid connection", "sql: database is closed", "broken pipe",
		"i/o timeout", "io timeout", "connection refused", "connection reset",
		"wsarecv", "eof", "bad connection", "going away", "lost connection",
		"too many connections",
	} {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}
