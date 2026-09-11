package schemacache

import (
	"sync"

	"gorm.io/gorm"
)

var cols sync.Map // table\x00col → bool

func HasColumn(db *gorm.DB, table, col string) bool {
	if db == nil || table == "" || col == "" {
		return false
	}
	key := table + "\x00" + col
	if v, ok := cols.Load(key); ok {
		return v.(bool)
	}
	var n int64
	if db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, col).Scan(&n).Error != nil {
		return false
	}
	ok := n > 0
	cols.Store(key, ok)
	return ok
}
