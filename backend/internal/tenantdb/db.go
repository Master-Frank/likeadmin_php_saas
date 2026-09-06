package tenantdb

import (
	"context"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ctxKey struct{}

// Shardable tables match server/app/platformapi/db/tenant.sql (la_* without prefix).
var shardable = map[string]struct{}{
	"tenant_admin": {}, "tenant_admin_dept": {}, "tenant_admin_jobs": {}, "tenant_admin_role": {},
	"tenant_admin_session": {}, "tenant_config": {}, "tenant_dept": {}, "tenant_file": {},
	"tenant_file_cate": {}, "tenant_jobs": {}, "tenant_notice_record": {}, "tenant_notice_setting": {},
	"tenant_pay_config": {}, "tenant_pay_way": {}, "tenant_system_menu": {}, "tenant_system_role": {},
	"tenant_system_role_menu": {}, "user": {}, "user_account_log": {}, "user_auth": {},
	"user_session": {}, "article": {}, "article_cate": {}, "decorate_page": {}, "decorate_tabbar": {},
}

// ShardableNames returns table bases that get a _{sn} suffix when tactics=1.
func ShardableNames() []string {
	out := make([]string, 0, len(shardable))
	for name := range shardable {
		out = append(out, name)
	}
	return out
}

var callbacksOnce bool

func Register(db *gorm.DB) {
	if db == nil || callbacksOnce {
		return
	}
	callbacksOnce = true
	_ = db.Callback().Query().Before("gorm:query").Register("likeadmin:shard", rewrite)
	_ = db.Callback().Create().Before("gorm:create").Register("likeadmin:shard_create", rewrite)
	_ = db.Callback().Update().Before("gorm:update").Register("likeadmin:shard_update", rewrite)
	_ = db.Callback().Delete().Before("gorm:delete").Register("likeadmin:shard_delete", rewrite)
	_ = db.Callback().Row().Before("gorm:row").Register("likeadmin:shard_row", rewrite)
}

func Use(c *gin.Context) *gorm.DB {
	if bootstrap.DB == nil {
		return nil
	}
	if c == nil {
		return bootstrap.DB
	}
	meta := ctxutil.Get(c)
	if meta.Tactics != 1 || meta.TenantSN == "" {
		return bootstrap.DB
	}
	return UseSN(meta.TenantSN)
}

// ForTenant returns the shard DB when la_tenant.tactics=1, otherwise the shared DB.
func ForTenant(tenantID uint) *gorm.DB {
	if bootstrap.DB == nil {
		return nil
	}
	if tenantID == 0 {
		return bootstrap.DB
	}
	var t model.Tenant
	if err := bootstrap.DB.Select("id", "sn", "tactics").Where("id = ?", tenantID).First(&t).Error; err != nil {
		return bootstrap.DB
	}
	if t.Tactics == 1 && t.SN != "" {
		return UseSN(t.SN)
	}
	return bootstrap.DB
}

func UseSN(sn string) *gorm.DB {
	if bootstrap.DB == nil {
		return nil
	}
	if sn == "" {
		return bootstrap.DB
	}
	return bootstrap.DB.WithContext(context.WithValue(context.Background(), ctxKey{}, sn))
}

func rewrite(db *gorm.DB) {
	if db == nil || db.Statement == nil || db.Statement.Context == nil {
		return
	}
	sn, _ := db.Statement.Context.Value(ctxKey{}).(string)
	if sn == "" {
		return
	}
	table := db.Statement.Table
	if table == "" && db.Statement.Schema != nil {
		table = db.Statement.Schema.Table
	}
	if table == "" || strings.Contains(table, "{") || strings.HasSuffix(table, "_"+sn) {
		return
	}
	base := strings.TrimPrefix(table, "la_")
	if _, ok := shardable[base]; !ok {
		return
	}
	db.Statement.Table = table + "_" + sn
}
