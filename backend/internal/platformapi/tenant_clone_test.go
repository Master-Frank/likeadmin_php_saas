package platformapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
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

func TestCopyTenantMenusRemapStampsUpdateTime(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990002
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantSystemMenu{})
	}
	cleanup()
	t.Cleanup(cleanup)
	before := time.Now().Unix() - 1
	if err := db.Transaction(func(tx *gorm.DB) error {
		return copyTenantMenus(tx, tid)
	}); err != nil {
		t.Fatal(err)
	}
	var child model.TenantSystemMenu
	if err := db.Where("tenant_id = ? AND pid > 0", tid).First(&child).Error; err != nil {
		t.Skip("no remapped child menus")
	}
	if child.UpdateTime == nil || *child.UpdateTime < before {
		t.Fatalf("remap update_time=%v", child.UpdateTime)
	}
}

func TestCopyTenantPayRemapOnClone(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990003
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantPayWay{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantPayConfig{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	var tplCfgs, tplWays int64
	db.Model(&model.TenantPayConfig{}).Where("tenant_id = 0").Count(&tplCfgs)
	db.Model(&model.TenantPayWay{}).Where("tenant_id = 0").Count(&tplWays)
	if tplCfgs == 0 || tplWays == 0 {
		t.Skip("no pay templates")
	}
	var tplIDs []uint
	db.Model(&model.TenantPayConfig{}).Where("tenant_id = 0").Pluck("id", &tplIDs)

	if err := db.Transaction(func(tx *gorm.DB) error {
		return copyTenantPay(tx, tid)
	}); err != nil {
		t.Fatal(err)
	}

	var cfgs []model.TenantPayConfig
	if err := db.Where("tenant_id = ?", tid).Find(&cfgs).Error; err != nil {
		t.Fatal(err)
	}
	if int64(len(cfgs)) != tplCfgs {
		t.Fatalf("pay configs %d want %d", len(cfgs), tplCfgs)
	}
	newIDs := map[uint]bool{}
	for _, c := range cfgs {
		newIDs[c.ID] = true
	}
	for _, old := range tplIDs {
		if newIDs[old] {
			t.Fatalf("cloned config reused template id %d", old)
		}
	}

	var ways []model.TenantPayWay
	if err := db.Where("tenant_id = ?", tid).Find(&ways).Error; err != nil {
		t.Fatal(err)
	}
	if int64(len(ways)) != tplWays {
		t.Fatalf("pay ways %d want %d", len(ways), tplWays)
	}
	for _, w := range ways {
		if !newIDs[w.PayConfigID] {
			t.Fatalf("way scene=%d pay_config_id=%d not remapped onto cloned configs", w.Scene, w.PayConfigID)
		}
	}
}

func TestInitSharedTenantChain(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990004
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Exec("DELETE ad FROM la_tenant_admin_dept ad INNER JOIN la_tenant_admin a ON ad.admin_id = a.id WHERE a.tenant_id = ?", tid).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantAdmin{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantDept{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantSystemMenu{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.Article{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.ArticleCate{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantPayWay{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantPayConfig{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.TenantNoticeSetting{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.DecoratePage{}).Error
		_ = db.Where("tenant_id = ?", tid).Delete(&model.DecorateTabbar{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/tenant.tenant/add",
		bytes.NewBufferString(`{"account":"t990004","password":"likeadmin"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	tenant := model.Tenant{ID: tid, SN: "t990004", Name: "clone-chain", Tactics: 0}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return initSharedTenant(tx, tenant, c)
	}); err != nil {
		t.Fatal(err)
	}

	var admins int64
	db.Model(&model.TenantAdmin{}).Where("tenant_id = ? AND account = ? AND root = 1 AND delete_time IS NULL", tid, "t990004").Count(&admins)
	if admins != 1 {
		t.Fatalf("super admin %d", admins)
	}
	var depts, menus, pays, ways, notices, pages, bars int64
	db.Model(&model.TenantDept{}).Where("tenant_id = ?", tid).Count(&depts)
	db.Model(&model.TenantSystemMenu{}).Where("tenant_id = ?", tid).Count(&menus)
	db.Model(&model.TenantPayConfig{}).Where("tenant_id = ?", tid).Count(&pays)
	db.Model(&model.TenantPayWay{}).Where("tenant_id = ?", tid).Count(&ways)
	db.Model(&model.TenantNoticeSetting{}).Where("tenant_id = ?", tid).Count(&notices)
	db.Model(&model.DecoratePage{}).Where("tenant_id = ?", tid).Count(&pages)
	db.Model(&model.DecorateTabbar{}).Where("tenant_id = ?", tid).Count(&bars)
	if depts < 1 || menus < 1 || pays < 3 || ways < 1 || notices < 1 || pages < 1 || bars < 1 {
		t.Fatalf("chain dept=%d menus=%d pay=%d/%d notice=%d page=%d bar=%d",
			depts, menus, pays, ways, notices, pages, bars)
	}
	var admin model.TenantAdmin
	db.Where("tenant_id = ? AND root = 1", tid).First(&admin)
	var link int64
	db.Table("la_tenant_admin_dept").Where("admin_id = ?", admin.ID).Count(&link)
	if link != 1 {
		t.Fatalf("admin_dept %d", link)
	}

	var pair1Menus int64
	db.Model(&model.TenantSystemMenu{}).Where("tenant_id = 1").Count(&pair1Menus)
	if pair1Menus == 0 {
		t.Fatal("must not wipe pair1 menus")
	}
}

func TestInitShardedTenantChain(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990005
	const sn = "t990005"
	db := bootstrap.DB
	cleanup := func() {
		dropShardedTenantTables(sn)
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	tenantdb.Register(db)
	if err := runTenantSQL(sn); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/tenant.tenant/add",
		bytes.NewBufferString(`{"account":"t990005","password":"likeadmin"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	tenant := model.Tenant{ID: tid, SN: sn, Name: "shard-chain", Tactics: 1}
	if err := initShardedTenant(db, tenant, c); err != nil {
		t.Fatal(err)
	}

	sdb := tenantdb.UseSN(sn)
	var admin model.TenantAdmin
	if err := sdb.Where("id = 1 AND tenant_id = ? AND delete_time IS NULL", tid).First(&admin).Error; err != nil {
		t.Fatalf("shard super admin: %v", err)
	}
	if admin.Account != "t990005" || admin.Root != 1 {
		t.Fatalf("admin %+v", admin)
	}
	var notices, arts, pays, menus, links int64
	sdb.Model(&model.TenantNoticeSetting{}).Where("tenant_id = ?", tid).Count(&notices)
	sdb.Model(&model.Article{}).Where("tenant_id = ?", tid).Count(&arts)
	sdb.Model(&model.TenantPayConfig{}).Where("tenant_id = ?", tid).Count(&pays)
	sdb.Model(&model.TenantSystemMenu{}).Where("tenant_id = ?", tid).Count(&menus)
	sdb.Table("la_tenant_admin_dept_" + sn).Where("admin_id = 1").Count(&links)
	if notices < 1 || arts < 1 || pays < 1 || menus < 1 || links != 1 {
		t.Fatalf("shard chain notice=%d article=%d pay=%d menu=%d admin_dept=%d",
			notices, arts, pays, menus, links)
	}

	var pair1Menus int64
	db.Model(&model.TenantSystemMenu{}).Where("tenant_id = 1").Count(&pair1Menus)
	if pair1Menus == 0 {
		t.Fatal("must not wipe pair1 menus")
	}
}

func TestTenantAdminDetailUsesShardTable(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990008
	const sn = "t990008"
	const adminID uint = 9900081
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Exec("DROP TABLE IF EXISTS la_tenant_admin_" + sn).Error
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	if err := db.Exec("CREATE TABLE la_tenant_admin_" + sn + " LIKE la_tenant_admin").Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	tenant := model.Tenant{ID: tid, SN: sn, Name: "shard-admin", Tactics: 1, CreateTime: now, UpdateTime: util.UnixPtr(now)}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	admin := newTenantSuperAdmin(adminID, tid, "shardadmin", "x", now)
	if err := tenantdb.UseSN(sn).Create(&admin).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tenant.tenant_admin/detail?id=9900081&tenant_id=990008", nil)
	TenantAdminDetail(c)

	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code != 1 {
		t.Fatalf("detail %+v body=%s", wrap, w.Body.String())
	}
	data, _ := wrap.Data.(map[string]any)
	if util.ToString(data["account"]) != "shardadmin" {
		t.Fatalf("account %v", data["account"])
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/platformapi/tenant.tenant_admin/edit",
		bytes.NewBufferString(`{"id":9900081,"tenant_id":990008,"name":"分表管理员","account":"shardadmin","multipoint_login":1}`))
	c2.Request.Header.Set("Content-Type", "application/json")
	TenantAdminEdit(c2)
	if err := json.Unmarshal(w2.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("edit json %s: %v", w2.Body.String(), err)
	}
	if wrap.Code != 1 {
		t.Fatalf("edit %+v body=%s", wrap, w2.Body.String())
	}
	var got model.TenantAdmin
	if err := tenantdb.UseSN(sn).Where("id = ?", adminID).First(&got).Error; err != nil || got.Name != "分表管理员" {
		t.Fatalf("edited %+v err=%v", got, err)
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
