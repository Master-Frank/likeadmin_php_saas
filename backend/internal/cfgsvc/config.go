package cfgsvc

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const (
	localKey = "likeadmin.cfg.local"
	cfgTTL   = 120 * time.Second
	missTTL  = 15 * time.Second
)

var (
	cfgSF    singleflight.Group
	errCfgDB = errors.New("config db unavailable")
)

type localEntry struct {
	miss      bool
	skipCache bool
	val       any
}

func db(c *gin.Context) *gorm.DB {
	if c != nil {
		return tenantdb.Use(c)
	}
	return bootstrap.DB
}

func platformDB(c *gin.Context) *gorm.DB {
	if bootstrap.DB == nil {
		return nil
	}
	if c != nil && c.Request != nil {
		return bootstrap.DB.WithContext(c.Request.Context())
	}
	return bootstrap.DB
}

func Get(c *gin.Context, typ, name string, defaultValue any) any {
	if e, ok := localGet(c, typ, name); ok {
		return applyDefault(e, defaultValue, typ, name)
	}
	usePlatform := usePlatformCfg(c, typ)
	tid := uint(0)
	if !usePlatform && c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	if !sensitiveCfg(typ, name) {
		if raw, ok := cache.Get(redisKey(usePlatform, tid, typ, name)); ok {
			e := decodeCached(raw)
			localSet(c, typ, name, e)
			return applyDefault(e, defaultValue, typ, name)
		}
	}
	e := loadCfg(c, usePlatform, tid, typ, name)
	if e.skipCache {
		return applyDefault(localEntry{miss: true}, defaultValue, typ, name)
	}
	localSet(c, typ, name, e)
	if !sensitiveCfg(typ, name) {
		storeRedis(usePlatform, tid, typ, name, e)
	}
	return applyDefault(e, defaultValue, typ, name)
}

func GetMany(c *gin.Context, typ string, names []string) map[string]any {
	out := make(map[string]any, len(names))
	missing := make([]string, 0, len(names))
	for _, name := range names {
		if e, ok := localGet(c, typ, name); ok {
			out[name] = applyDefault(e, nil, typ, name)
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) == 0 {
		return out
	}
	usePlatform := usePlatformCfg(c, typ)
	tid := uint(0)
	if !usePlatform && c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	still := make([]string, 0, len(missing))
	for _, name := range missing {
		if sensitiveCfg(typ, name) {
			still = append(still, name)
			continue
		}
		if raw, ok := cache.Get(redisKey(usePlatform, tid, typ, name)); ok {
			e := decodeCached(raw)
			localSet(c, typ, name, e)
			out[name] = applyDefault(e, nil, typ, name)
			continue
		}
		still = append(still, name)
	}
	if len(still) > 0 {
		loaded, err := loadMany(c, usePlatform, tid, typ, still)
		if err != nil {
			for _, name := range still {
				out[name] = applyDefault(localEntry{miss: true}, nil, typ, name)
			}
			return out
		}
		for _, name := range still {
			e, ok := loaded[name]
			if !ok {
				e = localEntry{miss: true}
			}
			localSet(c, typ, name, e)
			if !sensitiveCfg(typ, name) {
				storeRedis(usePlatform, tid, typ, name, e)
			}
			out[name] = applyDefault(e, nil, typ, name)
		}
	}
	return out
}

func Warm(c *gin.Context, typ string, names ...string) {
	_ = GetMany(c, typ, names)
}

func GetString(c *gin.Context, typ, name, def string) string {
	var defaultValue any
	if def != "" {
		defaultValue = def
	}
	v := Get(c, typ, name, defaultValue)
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
	usePlatform := meta.Source == ctxutil.SourcePlatform || typ == "storage"
	tid := meta.TenantID
	if usePlatform {
		tid = 0
		var row model.ConfigRow
		err := platformDB(c).Where("type = ? AND name = ?", typ, name).First(&row).Error
		if err != nil {
			platformDB(c).Create(&model.ConfigRow{Type: typ, Name: name, Value: s, CreateTime: now, UpdateTime: util.UnixPtr(now)})
		} else {
			platformDB(c).Model(&row).Updates(map[string]any{"value": s, "update_time": now})
		}
	} else {
		var row model.TenantConfig
		q := db(c).Where("type = ? AND name = ? AND tenant_id = ?", typ, name, tid)
		err := q.First(&row).Error
		if err != nil {
			db(c).Create(&model.TenantConfig{Type: typ, Name: name, Value: s, TenantID: tid, CreateTime: now, UpdateTime: util.UnixPtr(now)})
		} else {
			db(c).Model(&row).Updates(map[string]any{"value": s, "update_time": now})
		}
	}
	invalidateCfg(c, usePlatform, tid, typ, name)
	return raw
}

func invalidateCfg(c *gin.Context, usePlatform bool, tid uint, typ, name string) {
	if c != nil {
		if v, ok := c.Get(localKey); ok {
			if m, ok := v.(map[string]localEntry); ok {
				delete(m, localMapKey(typ, name))
			}
		}
	}
	cache.Del(redisKey(usePlatform, tid, typ, name))
	BumpBoot(tid)
}

func BumpBoot(tid uint) {
	cache.Incr(bootVerKey(tid))
	cache.DelPrefix("boot:" + strconv.FormatUint(uint64(tid), 10) + ":")
}

func BootVersion(tid uint) string {
	raw, ok := cache.Get(bootVerKey(tid))
	if !ok || raw == "" {
		return "0"
	}
	return strings.TrimSpace(raw)
}

func bootVerKey(tid uint) string {
	return "bootver:" + strconv.FormatUint(uint64(tid), 10)
}

func usePlatformCfg(c *gin.Context, typ string) bool {
	meta := ctxutil.Get(c)
	return meta.Source == ctxutil.SourcePlatform || typ == "storage"
}

func sensitiveCfg(typ, name string) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, "secret") || strings.Contains(n, "private") ||
		strings.Contains(n, "cert") || strings.Contains(n, "password") ||
		strings.Contains(n, "access_key") || strings.Contains(n, "mch_key") ||
		strings.Contains(n, "encoding_aes") {
		return true
	}
	if (typ == "oa_setting" || typ == "mnp_setting" || typ == "open_platform") && n == "token" {
		return true
	}
	if typ == "storage" && n != "default" && n != "local" {
		return true
	}
	if typ == "sms" {
		return true
	}
	return false
}

func redisKey(platform bool, tid uint, typ, name string) string {
	if platform {
		return "cfg:platform:" + typ + ":" + name
	}
	return "cfg:tenant:" + strconv.FormatUint(uint64(tid), 10) + ":" + typ + ":" + name
}

func localMapKey(typ, name string) string { return typ + "\x00" + name }

func localGet(c *gin.Context, typ, name string) (localEntry, bool) {
	if c == nil {
		return localEntry{}, false
	}
	v, ok := c.Get(localKey)
	if !ok {
		return localEntry{}, false
	}
	m, ok := v.(map[string]localEntry)
	if !ok {
		return localEntry{}, false
	}
	e, ok := m[localMapKey(typ, name)]
	return e, ok
}

func localSet(c *gin.Context, typ, name string, e localEntry) {
	if c == nil {
		return
	}
	m, ok := c.Get(localKey)
	var store map[string]localEntry
	if ok {
		store, _ = m.(map[string]localEntry)
	}
	if store == nil {
		store = map[string]localEntry{}
		c.Set(localKey, store)
	}
	store[localMapKey(typ, name)] = e
}

func applyDefault(e localEntry, defaultValue any, typ, name string) any {
	if e.miss {
		if defaultValue != nil {
			return defaultValue
		}
		return projectFallback(typ, name)
	}
	return e.val
}

func decodeCached(raw string) localEntry {
	if raw == "\x00miss" {
		return localEntry{miss: true}
	}
	return localEntry{val: decodeValue(raw)}
}

func storeRedis(platform bool, tid uint, typ, name string, e localEntry) {
	key := redisKey(platform, tid, typ, name)
	if e.miss {
		cache.Set(key, "\x00miss", missTTL)
		return
	}
	b, err := json.Marshal(e.val)
	if err != nil {
		return
	}
	cache.Set(key, string(b), cfgTTL)
}

func loadCfg(c *gin.Context, usePlatform bool, tid uint, typ, name string) localEntry {
	sfKey := redisKey(usePlatform, tid, typ, name)
	v, _, _ := cfgSF.Do(sfKey, func() (any, error) {
		if db(c) == nil && bootstrap.DB == nil {
			return localEntry{miss: true, skipCache: true}, nil
		}
		var value string
		var err error
		if usePlatform {
			if platformDB(c) == nil {
				return localEntry{miss: true, skipCache: true}, nil
			}
			err = platformDB(c).Where("type = ? AND name = ?", typ, name).Model(&model.ConfigRow{}).Select("value").Scan(&value).Error
		} else {
			q := db(c).Where("type = ? AND name = ?", typ, name)
			q = q.Where("tenant_id = ?", tid)
			err = q.Model(&model.TenantConfig{}).Select("value").Scan(&value).Error
		}
		if err != nil {
			return localEntry{miss: true, skipCache: true}, nil
		}
		if value == "" {
			return localEntry{miss: true}, nil
		}
		return localEntry{val: decodeValue(value)}, nil
	})
	if v == nil {
		return localEntry{miss: true, skipCache: true}
	}
	return v.(localEntry)
}

func loadMany(c *gin.Context, usePlatform bool, tid uint, typ string, names []string) (map[string]localEntry, error) {
	out := make(map[string]localEntry, len(names))
	for _, n := range names {
		out[n] = localEntry{miss: true}
	}
	if len(names) == 0 {
		return out, nil
	}
	if db(c) == nil && bootstrap.DB == nil {
		return out, errCfgDB
	}
	type row struct {
		Name  string
		Value string
	}
	var rows []row
	var err error
	if usePlatform {
		if platformDB(c) == nil {
			return out, errCfgDB
		}
		err = platformDB(c).Model(&model.ConfigRow{}).Select("name, value").
			Where("type = ? AND name IN ?", typ, names).Scan(&rows).Error
	} else {
		if db(c) == nil {
			return out, errCfgDB
		}
		err = db(c).Model(&model.TenantConfig{}).Select("name, value").
			Where("type = ? AND name IN ? AND tenant_id = ?", typ, names, tid).Scan(&rows).Error
	}
	if err != nil {
		return out, err
	}
	for _, r := range rows {
		if r.Value == "" {
			continue
		}
		out[r.Name] = localEntry{val: decodeValue(r.Value)}
	}
	return out, nil
}

func decodeValue(value string) any {
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
