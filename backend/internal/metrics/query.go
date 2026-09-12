package metrics

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type ctxKey struct{}

const GinKey = "likeadmin.query_stats"

type Stats struct {
	Count int64
	Ns    int64
}

func (s *Stats) Add(n int64, d time.Duration) {
	if s == nil {
		return
	}
	atomic.AddInt64(&s.Count, n)
	atomic.AddInt64(&s.Ns, d.Nanoseconds())
}

func (s *Stats) Snapshot() (count int64, dur time.Duration) {
	if s == nil {
		return 0, 0
	}
	return atomic.LoadInt64(&s.Count), time.Duration(atomic.LoadInt64(&s.Ns))
}

func WithStats(ctx context.Context, s *Stats) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, s)
}

func FromContext(ctx context.Context) *Stats {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(ctxKey{}).(*Stats)
	return s
}

func FromGin(c *gin.Context) *Stats {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(GinKey); ok {
		if s, ok := v.(*Stats); ok {
			return s
		}
	}
	if c.Request != nil {
		return FromContext(c.Request.Context())
	}
	return nil
}

func Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		st := &Stats{}
		c.Set(GinKey, st)
		if c.Request != nil {
			c.Request = c.Request.WithContext(WithStats(c.Request.Context(), st))
		}
		HTTPInFlightAdd(1)
		start := time.Now()
		c.Next()
		HTTPInFlightAdd(-1)
		ObserveHTTP(time.Since(start))
		AddHTTP()
	}
}

func Register(db *gorm.DB) {
	if db == nil {
		return
	}
	_ = db.Callback().Query().Before("gorm:query").Register("likeadmin:qstart", before)
	_ = db.Callback().Query().After("gorm:after_query").Register("likeadmin:qcount", after)
	_ = db.Callback().Create().Before("gorm:create").Register("likeadmin:qstart_create", before)
	_ = db.Callback().Create().After("gorm:after_create").Register("likeadmin:qcount_create", after)
	_ = db.Callback().Update().Before("gorm:update").Register("likeadmin:qstart_update", before)
	_ = db.Callback().Update().After("gorm:after_update").Register("likeadmin:qcount_update", after)
	_ = db.Callback().Delete().Before("gorm:delete").Register("likeadmin:qstart_delete", before)
	_ = db.Callback().Delete().After("gorm:after_delete").Register("likeadmin:qcount_delete", after)
	_ = db.Callback().Row().Before("gorm:row").Register("likeadmin:qstart_row", before)
	_ = db.Callback().Row().After("gorm:row").Register("likeadmin:qcount_row", after)
	_ = db.Callback().Raw().Before("gorm:raw").Register("likeadmin:qstart_raw", before)
	_ = db.Callback().Raw().After("gorm:raw").Register("likeadmin:qcount_raw", after)
}

func before(db *gorm.DB) {
	if db == nil {
		return
	}
	db.InstanceSet("likeadmin:qstart", time.Now())
}

func after(db *gorm.DB) {
	if db == nil || db.Statement == nil {
		return
	}
	elapsed := time.Duration(0)
	if v, ok := db.InstanceGet("likeadmin:qstart"); ok {
		if start, ok := v.(time.Time); ok {
			elapsed = time.Since(start)
		}
	}
	if st := FromContext(db.Statement.Context); st != nil {
		st.Add(1, elapsed)
	}
	AddSQL()
	ObserveSQL(elapsed)
}
