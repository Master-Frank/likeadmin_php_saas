package metrics

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"
)

func TestStatsAddAndFromContext(t *testing.T) {
	st := &Stats{}
	ctx := WithStats(context.Background(), st)
	FromContext(ctx).Add(2, 5*time.Millisecond)
	n, d := st.Snapshot()
	if n != 2 || d < 5*time.Millisecond {
		t.Fatalf("count=%d dur=%s", n, d)
	}
	if FromContext(context.Background()) != nil {
		t.Fatal("empty context")
	}
}

func TestGlobalSQLMetricsDoNotRequireRequestStats(t *testing.T) {
	beforeCount := sqlQueries.Load()
	db := &gorm.DB{Statement: &gorm.Statement{Context: context.Background()}}
	before(db)
	after(db)
	if sqlQueries.Load() != beforeCount+1 {
		t.Fatalf("global SQL count did not advance: before=%d after=%d", beforeCount, sqlQueries.Load())
	}
}
