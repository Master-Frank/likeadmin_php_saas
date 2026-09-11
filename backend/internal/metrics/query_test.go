package metrics

import (
	"context"
	"testing"
	"time"
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
