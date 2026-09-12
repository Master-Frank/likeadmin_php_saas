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
	_ = db.Callback().Row().After("gorm:row").Register("likeadmin:replica_retry_row", replicaRetryAfter)
	_ = db.Callback().Raw().After("gorm:raw").Register("likeadmin:replica_retry_raw", replicaRetryAfter)
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
