package schemacache

import (
	"sync"
	"time"

	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

type colHit struct {
	ok bool
	at time.Time
}

const colTTL = 5 * time.Minute

var cols sync.Map // db|table|col → colHit

func HasColumn(db *gorm.DB, table, col string) bool {
	if db == nil || table == "" || col == "" {
		return false
	}
	key := dbIdent() + "\x00" + table + "\x00" + col
	if v, ok := cols.Load(key); ok {
		h := v.(colHit)
		if time.Since(h.at) < colTTL {
			return h.ok
		}
	}
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, col).Scan(&n).Error; err != nil {
		return false
	}
	ok := n > 0
	cols.Store(key, colHit{ok: ok, at: time.Now()})
	return ok
}

func Invalidate() {
	cols.Range(func(k, _ any) bool {
		cols.Delete(k)
		return true
	})
}

func dbIdent() string {
	return config.C.Database.Hostname + "|" + config.C.Database.Database
}
