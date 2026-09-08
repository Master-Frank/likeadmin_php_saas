package model

import (
	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

func T(name string) string {
	return config.Prefix() + name
}

type SoftDelete struct {
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time,omitempty"`
}

func NotDeleted(db *gorm.DB) *gorm.DB {
	return db.Where("delete_time IS NULL")
}
