package cfgsvc

import (
	"encoding/json"
	"strconv"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func db(c *gin.Context) *gorm.DB {
	if c != nil {
		return tenantdb.Use(c)
	}
	return bootstrap.DB
}

func Get(c *gin.Context, typ, name string, defaultValue any) any {
	if db(c) == nil && bootstrap.DB == nil {
		if defaultValue != nil {
			return defaultValue
		}
		return projectFallback(typ, name)
	}
	query := db(c).Where("type = ? AND name = ?", typ, name)
	meta := ctxutil.Get(c)
	usePlatform := meta.Source == ctxutil.SourcePlatform || typ == "storage"
	var value string
	var err error
	if usePlatform {
		err = bootstrap.DB.Where("type = ? AND name = ?", typ, name).Model(&model.ConfigRow{}).Select("value").Scan(&value).Error
	} else {
		if meta.TenantID > 0 {
			query = query.Where("tenant_id = ?", meta.TenantID)
		}
		err = query.Model(&model.TenantConfig{}).Select("value").Scan(&value).Error
	}
	if err != nil || value == "" {
		if err == gorm.ErrRecordNotFound || value == "" {
			if defaultValue != nil {
				return defaultValue
			}
			return projectFallback(typ, name)
		}
	}
	var decoded any
	if json.Unmarshal([]byte(value), &decoded) == nil {
		return decoded
	}
	if value == "0" {
		return 0
	}
	if n, e := strconv.Atoi(value); e == nil {
		return n
	}
	return value
}

func GetString(c *gin.Context, typ, name, def string) string {
	v := Get(c, typ, name, def)
	if v == nil {
		return def
	}
	return util.ToString(v)
}

func GetInt(c *gin.Context, typ, name string, def int) int {
	v := Get(c, typ, name, def)
	if v == nil {
		return def
	}
	return util.ToInt(v)
}

func Set(c *gin.Context, typ, name string, value any) any {
	raw := value
	s := util.ToString(value)
	if _, ok := value.(map[string]any); ok {
		b, _ := json.Marshal(value)
		s = string(b)
	}
	if _, ok := value.([]any); ok {
		b, _ := json.Marshal(value)
		s = string(b)
	}
	meta := ctxutil.Get(c)
	now := util.NowUnix()
	if meta.Source == ctxutil.SourcePlatform {
		var row model.ConfigRow
		err := bootstrap.DB.Where("type = ? AND name = ?", typ, name).First(&row).Error
		if err != nil {
			bootstrap.DB.Create(&model.ConfigRow{Type: typ, Name: name, Value: s, CreateTime: now})
		} else {
			bootstrap.DB.Model(&row).Updates(map[string]any{"value": s, "update_time": now})
		}
		return raw
	}
	var row model.TenantConfig
	q := db(c).Where("type = ? AND name = ?", typ, name)
	if meta.TenantID > 0 {
		q = q.Where("tenant_id = ?", meta.TenantID)
	}
	err := q.First(&row).Error
	if err != nil {
		db(c).Create(&model.TenantConfig{Type: typ, Name: name, Value: s, TenantID: meta.TenantID, CreateTime: now})
	} else {
		db(c).Model(&row).Updates(map[string]any{"value": s, "update_time": now})
	}
	return raw
}

func projectFallback(typ, name string) any {
	switch typ {
	case "platform":
		if v, ok := config.C.Project.Platform[name]; ok {
			return v
		}
	case "tenant":
		if v, ok := config.C.Project.Tenant[name]; ok {
			return v
		}
	case "admin_login":
		switch name {
		case "login_restrictions":
			return 1
		case "password_error_times":
			return 5
		case "limit_login_time":
			return 30
		}
	case "website":
		if v, ok := config.C.Project.Website[name]; ok {
			return v
		}
	case "login":
		if v, ok := config.C.Project.Login[name]; ok {
			return v
		}
	case "decorate":
		if v, ok := config.C.Project.Decorate[name]; ok {
			return v
		}
	}
	return nil
}
