package dbindex

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/schemacache"

	"gorm.io/gorm"
)

type Item struct {
	Table      string `json:"table"`
	Name       string `json:"name"`
	Cols       string `json:"cols"`
	Present    bool   `json:"present"`
	Action     string `json:"action"`
	LockImpact string `json:"lock_impact"`
	Error      string `json:"error,omitempty"`
}

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
		{table: p + "operation_log", name: "idx_tenant_create_id", cols: "`tenant_id`,`create_time`,`id`"},
	}
}

// EnsurePerfIndexes creates the P0/P1 lookup indexes if missing. Call from
// install or `think ensure-indexes`; HTTP startup does not run this unless
// LIKEADMIN_ENSURE_INDEXES=1.
func EnsurePerfIndexes(db *gorm.DB) {
	if db == nil {
		return
	}
	if os.Getenv("LIKEADMIN_REQUIRE_DDL") == "0" {
		return
	}
	ensureOperationLogTenantID(db)
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

func lockNote() string {
	return "MySQL 8 secondary INDEX is typically INPLACE with a brief metadata lock; run against large tables in a maintenance window."
}

// Plan reports each candidate index without modifying schema.
func Plan(db *gorm.DB) []Item {
	out := make([]Item, 0, 16)
	for _, s := range specs() {
		tables := []string{s.table}
		if db != nil && s.shards {
			tables = append(tables, shardTables(db, s.table)...)
		}
		seen := map[string]bool{}
		for _, table := range tables {
			if table == "" || seen[table] {
				continue
			}
			seen[table] = true
			it := Item{Table: table, Name: s.name, Cols: s.cols, LockImpact: lockNote(), Action: "create"}
			if db == nil {
				it.Action = "skipped"
				it.Error = "database unavailable"
				out = append(out, it)
				continue
			}
			if !tableExists(db, table) {
				it.Action = "skipped"
				it.Error = "table missing"
				out = append(out, it)
				continue
			}
			if hasIndex(db, table, s.name) {
				it.Present = true
				it.Action = "exists"
			}
			out = append(out, it)
		}
	}
	return out
}

// Missing returns indexes that should exist but do not.
func Missing(db *gorm.DB) []Item {
	if db == nil {
		return nil
	}
	var out []Item
	for _, it := range Plan(db) {
		if !it.Present && it.Action == "create" {
			out = append(out, it)
		}
	}
	return out
}

func FormatPlan(items []Item) string {
	if len(items) == 0 {
		return "no index plan"
	}
	var b strings.Builder
	for _, it := range items {
		status := it.Action
		if it.Present {
			status = "present"
		}
		fmt.Fprintf(&b, "%s.%s (%s) %s", it.Table, it.Name, it.Cols, status)
		if it.Error != "" {
			fmt.Fprintf(&b, " [%s]", it.Error)
		}
		b.WriteByte('\n')
		fmt.Fprintf(&b, "  lock: %s\n", it.LockImpact)
	}
	return b.String()
}

func StatusPath() string {
	if config.C.App.PublicDir != "" {
		return filepath.Join(filepath.Dir(config.C.App.PublicDir), "runtime", "index-status.json")
	}
	return filepath.Join("runtime", "index-status.json")
}

func WriteStatus(items []Item, runErr error) {
	path := StatusPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o775)
	payload := map[string]any{
		"time":  time.Now().Unix(),
		"items": items,
	}
	if runErr != nil {
		payload["error"] = runErr.Error()
	}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644)
}

func ensureOperationLogTenantID(db *gorm.DB) {
	table := config.Prefix() + "operation_log"
	if !tableExists(db, table) || hasColumn(db, table, "tenant_id") {
		return
	}
	sql := "ALTER TABLE `" + table + "` ADD COLUMN `tenant_id` int NOT NULL DEFAULT 0 COMMENT '租户ID'"
	if err := db.Exec(sql).Error; err != nil {
		log.Printf("operation_log tenant_id: %v", err)
		return
	}
	schemacache.Invalidate()
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

func hasColumn(db *gorm.DB, table, col string) bool {
	var n int64
	if db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, col).Scan(&n).Error != nil {
		return false
	}
	return n > 0
}
