package platformapi

import (
	"os"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"

	"gorm.io/gorm"
)

func initTenantCloneDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		return true
	}
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if err := bootstrap.Init(cfg); err != nil {
		t.Log(err)
		return false
	}
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
	}
	return bootstrap.DB != nil
}

func TestCopyTenantStampsUpdateTime(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990001
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Exec("DELETE ad FROM la_tenant_admin_dept ad INNER JOIN la_tenant_dept d ON ad.dept_id = d.id WHERE d.tenant_id = ?", tid).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.Article{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.ArticleCate{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantDept{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.DecoratePage{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.DecorateTabbar{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantNoticeSetting{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	before := time.Now().Unix() - 2
	if err := db.Transaction(func(tx *gorm.DB) error {
		if err := copyTenantDept(tx, tid, 0); err != nil {
			return err
		}
		if err := copyTenantArticles(tx, tid); err != nil {
			return err
		}
		if err := copyTenantNotice(tx, tid); err != nil {
			return err
		}
		return copyTenantDecorate(tx, tid)
	}); err != nil {
		t.Fatal(err)
	}

	var dept model.TenantDept
	if err := db.Where("tenant_id = ?", tid).First(&dept).Error; err != nil {
		t.Fatal(err)
	}
	assertFreshTimes(t, "dept", dept.CreateTime, dept.UpdateTime, before)

	var cate model.ArticleCate
	if err := db.Where("tenant_id = ?", tid).First(&cate).Error; err != nil {
		t.Fatal(err)
	}
	assertFreshTimes(t, "cate", cate.CreateTime, cate.UpdateTime, before)

	var art model.Article
	if err := db.Where("tenant_id = ?", tid).First(&art).Error; err != nil {
		t.Fatal(err)
	}
	assertFreshTimes(t, "article", art.CreateTime, art.UpdateTime, before)

	var page model.DecoratePage
	if err := db.Where("tenant_id = ?", tid).First(&page).Error; err != nil {
		t.Fatal(err)
	}
	assertFreshTimes(t, "page", page.CreateTime, page.UpdateTime, before)

	var bar model.DecorateTabbar
	if err := db.Where("tenant_id = ?", tid).First(&bar).Error; err != nil {
		t.Fatal(err)
	}
	assertFreshTimes(t, "tabbar", bar.CreateTime, bar.UpdateTime, before)

	var notice model.TenantNoticeSetting
	if err := db.Where("tenant_id = ?", tid).First(&notice).Error; err != nil {
		t.Fatal(err)
	}
	if notice.UpdateTime == nil || *notice.UpdateTime < before {
		t.Fatalf("notice update_time leaked template: %+v", notice.UpdateTime)
	}
}

func assertFreshTimes(t *testing.T, name string, create int64, update *int64, before int64) {
	t.Helper()
	if create < before {
		t.Fatalf("%s create_time=%d want >= %d", name, create, before)
	}
	if update == nil {
		t.Fatalf("%s update_time nil", name)
	}
	if *update != create {
		t.Fatalf("%s update_time=%d create_time=%d (template time leaked)", name, *update, create)
	}
}
