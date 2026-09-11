package tenantdb

import (
	"encoding/json"
	"strconv"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/model"

	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
)

const (
	tenantTTL    = 120 * time.Second
	negativeTTL  = 10 * time.Second
	missSentinel = `{"found":false}`
)

var tenantSF singleflight.Group

type cachedTenant struct {
	Found             bool   `json:"found"`
	ID                uint   `json:"id,omitempty"`
	SN                string `json:"sn,omitempty"`
	Tactics           int    `json:"tactics,omitempty"`
	Disable           int    `json:"disable,omitempty"`
	DomainAlias       string `json:"domain_alias,omitempty"`
	DomainAliasEnable int    `json:"domain_alias_enable,omitempty"`
}

func (c cachedTenant) Tenant() model.Tenant {
	return model.Tenant{
		ID: c.ID, SN: c.SN, Tactics: c.Tactics, Disable: c.Disable,
		DomainAlias: c.DomainAlias, DomainAliasEnable: c.DomainAliasEnable,
	}
}

func hostKey(host string) string { return "tenant:host:" + host }
func snKey(sn string) string     { return "tenant:sn:" + sn }
func idKey(id uint) string       { return "tenant:id:" + strconv.FormatUint(uint64(id), 10) }

func ByHost(host string) (model.Tenant, bool) {
	if host == "" {
		return model.Tenant{}, false
	}
	return lookup(hostKey(host), func() (cachedTenant, error) {
		var t model.Tenant
		err := platformDB(nil).Where("domain_alias = ? AND delete_time IS NULL", host).First(&t).Error
		if err != nil {
			return cachedTenant{Found: false}, err
		}
		return fromModel(t), nil
	})
}

func BySN(sn string) (model.Tenant, bool) {
	if sn == "" {
		return model.Tenant{}, false
	}
	return lookup(snKey(sn), func() (cachedTenant, error) {
		var t model.Tenant
		err := platformDB(nil).Where("sn = ? AND delete_time IS NULL", sn).First(&t).Error
		if err != nil {
			return cachedTenant{Found: false}, err
		}
		return fromModel(t), nil
	})
}

func ByID(id uint) (model.Tenant, bool) {
	if id == 0 {
		return model.Tenant{}, false
	}
	return lookup(idKey(id), func() (cachedTenant, error) {
		var t model.Tenant
		err := platformDB(nil).Where("id = ? AND delete_time IS NULL", id).First(&t).Error
		if err != nil {
			return cachedTenant{Found: false}, err
		}
		return fromModel(t), nil
	})
}

func lookup(key string, load func() (cachedTenant, error)) (model.Tenant, bool) {
	if bootstrap.DB == nil {
		return model.Tenant{}, false
	}
	if raw, ok := cache.Get(key); ok {
		var ct cachedTenant
		if json.Unmarshal([]byte(raw), &ct) == nil {
			if !ct.Found {
				return model.Tenant{}, false
			}
			return ct.Tenant(), true
		}
	}
	v, err, _ := tenantSF.Do(key, func() (any, error) {
		if raw, ok := cache.Get(key); ok {
			var ct cachedTenant
			if json.Unmarshal([]byte(raw), &ct) == nil {
				return ct, nil
			}
		}
		ct, err := load()
		if err != nil {
			ct = cachedTenant{Found: false}
			cache.Set(key, missSentinel, negativeTTL)
			return ct, nil
		}
		storeTenant(ct)
		return ct, nil
	})
	if err != nil || v == nil {
		return model.Tenant{}, false
	}
	ct := v.(cachedTenant)
	if !ct.Found {
		return model.Tenant{}, false
	}
	return ct.Tenant(), true
}

func fromModel(t model.Tenant) cachedTenant {
	return cachedTenant{
		Found: true, ID: t.ID, SN: t.SN, Tactics: t.Tactics, Disable: t.Disable,
		DomainAlias: t.DomainAlias, DomainAliasEnable: t.DomainAliasEnable,
	}
}

func storeTenant(ct cachedTenant) {
	if !ct.Found {
		return
	}
	raw, _ := json.Marshal(ct)
	s := string(raw)
	cache.Set(idKey(ct.ID), s, tenantTTL)
	if ct.SN != "" {
		cache.Set(snKey(ct.SN), s, tenantTTL)
	}
	if ct.DomainAlias != "" {
		cache.Set(hostKey(ct.DomainAlias), s, tenantTTL)
	}
}

// InvalidateTenant drops host/sn/id keys for the given row (old and new values).
func InvalidateTenant(rows ...model.Tenant) {
	for _, t := range rows {
		if t.ID > 0 {
			cache.Del(idKey(t.ID))
		}
		if t.SN != "" {
			cache.Del(snKey(t.SN))
		}
		if t.DomainAlias != "" {
			cache.Del(hostKey(t.DomainAlias))
		}
	}
}

func InvalidateHost(host string) {
	if host != "" {
		cache.Del(hostKey(host))
	}
}

func platformDB(session *gorm.DB) *gorm.DB {
	if session != nil {
		return session
	}
	return bootstrap.DB
}
