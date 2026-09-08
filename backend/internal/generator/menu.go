package generator

import (
	"strconv"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"

	"gorm.io/gorm"
)

// IsTenantModule is true when generated CRUD belongs to the tenant admin.
func IsTenantModule(t model.GenerateTable) bool {
	return goModuleApp(t.ModuleName) == "tenantapi"
}

func platformMenuTable() string {
	return config.Prefix() + "system_menu"
}

func tenantMenuTable(sn string) string {
	name := config.Prefix() + "tenant_system_menu"
	if sn != "" {
		return name + "_" + sn
	}
	return name
}

// RewriteMenuSQLForTenant rewrites platform la_system_menu SQL onto
// la_tenant_system_menu (optionally _{sn}) and injects tenant_id so the
// generated CRUD appears in the tenant admin, not only the platform menu
// table PHP writes.
func RewriteMenuSQLForTenant(sqlText, table string, tenantID uint) string {
	if strings.TrimSpace(sqlText) == "" {
		return ""
	}
	if table == "" {
		table = tenantMenuTable("")
	}
	src := platformMenuTable()
	out := strings.ReplaceAll(sqlText, "`"+src+"`", "`"+table+"`")
	out = strings.ReplaceAll(out, src, table)
	out = strings.ReplaceAll(out, "(`pid`,", "(`tenant_id`, `pid`,")
	id := strconv.FormatUint(uint64(tenantID), 10)
	out = strings.ReplaceAll(out, "VALUES (", "VALUES ("+id+", ")
	out = strings.ReplaceAll(out, "VALUES(", "VALUES("+id+", ")
	return out
}

// ApplyTenantMenus writes generated menu SQL to the tenant_id=0 template
// (copyTenantMenus / upgradeMenu copy from there) and to every live tenant,
// including tactics=1 shards that use la_tenant_system_menu_{sn}.
func ApplyTenantMenus(db *gorm.DB, sqlText string) error {
	if db == nil || strings.TrimSpace(sqlText) == "" {
		return nil
	}
	var tenants []model.Tenant
	if err := db.Where("delete_time IS NULL").Find(&tenants).Error; err != nil {
		return err
	}
	return applyTenantMenusTo(db, sqlText, tenants)
}

func applyTenantMenusTo(db *gorm.DB, sqlText string, tenants []model.Tenant) error {
	if db == nil || strings.TrimSpace(sqlText) == "" {
		return nil
	}
	tpl := tenantMenuTable("")
	if err := ApplyMenuSQL(db, RewriteMenuSQLForTenant(sqlText, tpl, 0)); err != nil {
		return err
	}
	for _, t := range tenants {
		table := tpl
		if t.Tactics == 1 && t.SN != "" {
			table = tenantMenuTable(t.SN)
		}
		if !sqlTableExists(db, table) {
			continue
		}
		if err := ApplyMenuSQL(db, RewriteMenuSQLForTenant(sqlText, table, t.ID)); err != nil {
			return err
		}
	}
	return nil
}

func sqlTableExists(db *gorm.DB, name string) bool {
	if db == nil || name == "" {
		return false
	}
	var n int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = ?",
		name,
	).Scan(&n).Error; err != nil {
		return false
	}
	return n > 0
}
