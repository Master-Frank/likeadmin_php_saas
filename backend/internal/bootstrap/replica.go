package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

var replicaRetryOnce sync.Once

func replicaMaxLag() time.Duration {
	s := strings.TrimSpace(os.Getenv("LIKEADMIN_REPLICA_MAX_LAG"))
	if s == "" {
		return 30 * time.Second
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 30 * time.Second
	}
	return time.Duration(n) * time.Second
}

func startReplicaRetry() {
	if len(config.C.Database.ReplicaList()) == 0 {
		return
	}
	replicaRetryOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if len(config.C.Database.ReplicaList()) == 0 || DB == nil {
					continue
				}
				if ReadDB == nil || ReadDB == DB {
					bindReadDB(DB)
				}
			}
		}()
	})
}

func replicaLag(db *gorm.DB) (time.Duration, bool) {
	if db == nil || db == DB || db.Config == nil {
		return 0, false
	}
	sqlDB, err := db.DB()
	if err != nil {
		return 0, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for _, q := range []string{"SHOW REPLICA STATUS", "SHOW SLAVE STATUS"} {
		d, ok := scanReplicaLag(ctx, sqlDB, q)
		if ok {
			return d, true
		}
	}
	return 0, false
}

func scanReplicaLag(ctx context.Context, sqlDB *sql.DB, query string) (time.Duration, bool) {
	rows, err := sqlDB.QueryContext(ctx, query)
	if err != nil {
		return 0, false
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil || !rows.Next() {
		return 0, false
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return 0, false
	}
	fields := map[string]any{}
	for i, c := range cols {
		fields[strings.ToLower(c)] = vals[i]
	}
	if ioRun := asString(fields["replica_io_running"]); ioRun == "" {
		ioRun = asString(fields["slave_io_running"])
		if ioRun != "" && !strings.EqualFold(ioRun, "yes") {
			return replicaMaxLag() + time.Second, true
		}
	} else if !strings.EqualFold(ioRun, "yes") {
		return replicaMaxLag() + time.Second, true
	}
	if sqlRun := asString(fields["replica_sql_running"]); sqlRun == "" {
		sqlRun = asString(fields["slave_sql_running"])
		if sqlRun != "" && !strings.EqualFold(sqlRun, "yes") {
			return replicaMaxLag() + time.Second, true
		}
	} else if !strings.EqualFold(sqlRun, "yes") {
		return replicaMaxLag() + time.Second, true
	}
	if d, ok := asSeconds(fields["seconds_behind_source"]); ok {
		return d, true
	}
	if d, ok := asSeconds(fields["seconds_behind_master"]); ok {
		return d, true
	}
	return 0, false
}

func asString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return strings.TrimSpace(strconv.FormatInt(asInt64(t), 10))
	}
}

func asInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case uint32:
		return int64(t)
	case uint64:
		return int64(t)
	default:
		return 0
	}
}

func asSeconds(v any) (time.Duration, bool) {
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case int64:
		return time.Duration(t) * time.Second, true
	case int32:
		return time.Duration(t) * time.Second, true
	case uint32:
		return time.Duration(t) * time.Second, true
	case uint64:
		return time.Duration(t) * time.Second, true
	case []byte:
		n, err := strconv.ParseInt(strings.TrimSpace(string(t)), 10, 64)
		if err != nil {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(t), 10, 64)
		if err != nil {
			return 0, false
		}
		return time.Duration(n) * time.Second, true
	default:
		return 0, false
	}
}
