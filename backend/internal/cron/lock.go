package cron

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
)

// withAdvisoryLock holds a MySQL named lock on one dedicated connection.
// It serializes schedulers across API/worker processes and remains held while
// the job runs. Non-MySQL test databases fall back to the caller's CAS.
func withAdvisoryLock(name string, fn func()) bool {
	if bootstrap.DB == nil || fn == nil {
		return false
	}
	if bootstrap.DB.Dialector.Name() != "mysql" {
		fn()
		return true
	}
	sqlDB, err := bootstrap.DB.DB()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return false
	}
	defer conn.Close()

	lockName := fmt.Sprintf("likeadmin:%s:%s", config.C.Database.Database, name)
	if len(lockName) > 64 {
		sum := fmt.Sprintf("%x", sha256.Sum256([]byte(lockName)))
		lockName = "likeadmin:" + sum[:54]
	}
	var acquired sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", lockName).Scan(&acquired); err != nil || !acquired.Valid || acquired.Int64 != 1 {
		return false
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer releaseCancel()
		_, _ = conn.ExecContext(releaseCtx, "SELECT RELEASE_LOCK(?)", lockName)
	}()
	fn()
	return true
}
