package dbindex

import (
	"fmt"
	"os"
	"strings"

	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

type ExplainQuery struct {
	Name string
	SQL  string
}

func ExplainQueries() []ExplainQuery {
	p := config.Prefix()
	return []ExplainQuery{
		{Name: "tenant_by_sn", SQL: "SELECT `id` FROM `" + p + "tenant` WHERE `sn` = ? AND `delete_time` IS NULL LIMIT 1"},
		{Name: "tenant_by_domain", SQL: "SELECT `id` FROM `" + p + "tenant` WHERE `domain_alias` = ? AND `delete_time` IS NULL LIMIT 1"},
		{Name: "config_by_type_name", SQL: "SELECT `id` FROM `" + p + "config` WHERE `type` = ? AND `name` = ? LIMIT 1"},
		{Name: "tenant_config", SQL: "SELECT `id` FROM `" + p + "tenant_config` WHERE `tenant_id` = ? AND `type` = ? AND `name` = ? LIMIT 1"},
		{Name: "article_show", SQL: "SELECT `id` FROM `" + p + "article` WHERE `tenant_id` = ? AND `is_show` = 1 AND `delete_time` IS NULL ORDER BY `id` DESC LIMIT 20"},
		{Name: "user_by_tenant", SQL: "SELECT `id` FROM `" + p + "user` WHERE `tenant_id` = ? AND `delete_time` IS NULL ORDER BY `id` DESC LIMIT 20"},
		{Name: "oplog_by_time", SQL: "SELECT `id` FROM `" + p + "operation_log` WHERE `create_time` >= ? ORDER BY `id` DESC LIMIT 20"},
		{Name: "oplog_by_tenant", SQL: "SELECT `id` FROM `" + p + "operation_log` WHERE `tenant_id` = ? AND `create_time` >= ? ORDER BY `id` DESC LIMIT 20"},
	}
}

// RunExplain prints EXPLAIN (or EXPLAIN ANALYZE when LIKEADMIN_EXPLAIN_ANALYZE=1)
// for the first-batch index query shapes. Results come from the connected DB only.
func RunExplain(db *gorm.DB) string {
	if db == nil {
		return "database unavailable"
	}
	analyze := os.Getenv("LIKEADMIN_EXPLAIN_ANALYZE") == "1"
	prefix := "EXPLAIN"
	if analyze {
		prefix = "EXPLAIN ANALYZE"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s on connected database (not a published capacity result)\n", prefix)
	for _, q := range ExplainQueries() {
		sql := prefix + " " + q.SQL
		rows, err := db.Raw(sql, explainArgs(q.SQL)...).Rows()
		fmt.Fprintf(&b, "\n-- %s\n%s\n", q.Name, sql)
		if err != nil {
			fmt.Fprintf(&b, "error: %v\n", err)
			continue
		}
		cols, _ := rows.Columns()
		fmt.Fprintf(&b, "%s\n", strings.Join(cols, " | "))
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				fmt.Fprintf(&b, "scan: %v\n", err)
				break
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				switch t := v.(type) {
				case []byte:
					parts[i] = string(t)
				default:
					parts[i] = fmt.Sprint(t)
				}
			}
			fmt.Fprintf(&b, "%s\n", strings.Join(parts, " | "))
		}
		_ = rows.Close()
	}
	return b.String()
}

func explainArgs(sql string) []any {
	n := strings.Count(sql, "?")
	out := make([]any, n)
	for i := 0; i < n; i++ {
		out[i] = 1
	}
	return out
}
