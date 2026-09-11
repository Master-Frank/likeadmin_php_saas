package dbindex

import (
	"log"
	"strings"
	"unicode"

	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

type spec struct {
	table  string
	name   string
	cols   string
	shards bool
}

func specs() []spec {
	p := config.Prefix()
	return []spec{
		{table: p + "tenant", name: "idx_sn_delete_time", cols: "`sn`,`delete_time`"},
		{table: p + "tenant", name: "idx_domain_alias_delete_time", cols: "`domain_alias`,`delete_time`"},
		{table: p + "config", name: "idx_type_name", cols: "`type`,`name`"},
		{table: p + "tenant_config", name: "idx_tenant_type_name", cols: "`tenant_id`,`type`,`name`", shards: true},
		{table: p + "article", name: "idx_tenant_show_delete", cols: "`tenant_id`,`is_show`,`delete_time`", shards: true},
		{table: p + "user", name: "idx_tenant_delete_time", cols: "`tenant_id`,`delete_time`", shards: true},
		{table: p + "operation_log", name: "idx_create_time", cols: "`create_time`"},
	}
}

// EnsurePerfIndexes creates the P0/P1 lookup indexes if missing. Safe to run
// on every boot; it inspects information_schema and skips existing names.
func EnsurePerfIndexes(db *gorm.DB) {
	if db == nil {
		return
	}
	for _, s := range specs() {
		tables := []string{s.table}
		if s.shards {
			tables = append(tables, shardTables(db, s.table)...)
		}
		seen := map[string]bool{}
		for _, table := range tables {
			if table == "" || seen[table] {
				continue
			}
			seen[table] = true
			if !tableExists(db, table) {
				continue
			}
			if hasIndex(db, table, s.name) {
				continue
			}
			sql := "CREATE INDEX `" + s.name + "` ON `" + table + "` (" + s.cols + ")"
			if err := db.Exec(sql).Error; err != nil {
				log.Printf("perf index %s on %s: %v", s.name, table, err)
			}
		}
	}
}

func shardTables(db *gorm.DB, base string) []string {
	var names []string
	if err := db.Raw("SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME LIKE ?", base+"_%").Scan(&names).Error; err != nil {
		return nil
	}
	out := make([]string, 0, len(names))
	for _, n := range names {
		if isShardCopy(n, base) {
			out = append(out, n)
		}
	}
	return out
}

func isShardCopy(table, base string) bool {
	if table == base {
		return true
	}
	prefix := base + "_"
	if !strings.HasPrefix(table, prefix) {
		return false
	}
	rest := strings.TrimPrefix(table, prefix)
	if rest == "" {
		return false
	}
	switch {
	case strings.HasPrefix(base, config.Prefix()+"article"):
		if rest == "cate" || strings.HasPrefix(rest, "cate_") || rest == "collect" || strings.HasPrefix(rest, "collect_") {
			return false
		}
	case strings.HasPrefix(base, config.Prefix()+"user"):
		if rest == "account_log" || strings.HasPrefix(rest, "account_log_") ||
			rest == "auth" || strings.HasPrefix(rest, "auth_") ||
			rest == "session" || strings.HasPrefix(rest, "session_") {
			return false
		}
	}
	for _, r := range rest {
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func tableExists(db *gorm.DB, table string) bool {
	var n int64
	if db.Raw("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", table).Scan(&n).Error != nil {
		return false
	}
	return n > 0
}

func hasIndex(db *gorm.DB, table, name string) bool {
	var n int64
	if db.Raw("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND INDEX_NAME = ?", table, name).Scan(&n).Error != nil {
		return false
	}
	return n > 0
}
