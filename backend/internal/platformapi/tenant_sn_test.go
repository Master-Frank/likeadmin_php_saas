package platformapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func TestRemapTenantPayConfigID(t *testing.T) {
	oldToNew := map[uint]uint{5: 11, 6: 12}
	wayToID := map[int]uint{1: 11, 2: 12, 3: 13}
	if got := remapTenantPayConfigID(5, oldToNew, wayToID, 1); got != 11 {
		t.Fatalf("old id %d", got)
	}
	if got := remapTenantPayConfigID(2, oldToNew, wayToID, 1); got != 12 {
		t.Fatalf("pay_way match %d", got)
	}
	if got := remapTenantPayConfigID(0, oldToNew, wayToID, 3); got != 13 {
		t.Fatalf("scene fallback %d", got)
	}
	if got := remapTenantPayConfigID(99, oldToNew, wayToID, 1); got != 99 {
		t.Fatalf("keep unknown %d", got)
	}
}

func TestValidTenantSN(t *testing.T) {
	if !validTenantSN("pair2") || !validTenantSN("abc123") {
		t.Fatal("valid sn rejected")
	}
	if !validTenantSN("UPPER") {
		t.Fatal("uppercase sn rejected")
	}
	if validTenantSN("") || validTenantSN("bad-sn") || validTenantSN("a_b") {
		t.Fatal("invalid sn accepted")
	}
}

func TestCleanTenantScopedRowsSkipsZero(t *testing.T) {
	cleanTenantScopedRows(0)
	expireTenantAdmins(model.Tenant{})
}

func TestApplyTenantSQLPlaceholders(t *testing.T) {
	oldPrefix := config.C.Database.Prefix
	oldDB := config.C.Database.Database
	config.C.Database.Prefix = "lk_"
	config.C.Database.Database = "testdb"
	defer func() {
		config.C.Database.Prefix = oldPrefix
		config.C.Database.Database = oldDB
	}()
	got := applyTenantSQLPlaceholders("INSERT INTO `la_article_{tenantSn}` VALUES ({tenantId});", "pair2", 9)
	if got != "INSERT INTO testdb.`lk_article_pair2` VALUES (9);" {
		t.Fatalf("%s", got)
	}
}

func TestTenantSQLDatabaseDefault(t *testing.T) {
	old := config.C.Database.Database
	config.C.Database.Database = ""
	defer func() { config.C.Database.Database = old }()
	if tenantSQLDatabase() != "likeadmin_saas" {
		t.Fatalf("%s", tenantSQLDatabase())
	}
}

func TestCanonicalizeNoticeJSON(t *testing.T) {
	raw := canonicalizeNoticeJSON(`{"status":1,"content":"hi"}`)
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil || int(m["status"].(float64)) != 1 {
		t.Fatalf("%s", raw)
	}
	dbl := canonicalizeNoticeJSON(`"{\"status\":0}"`)
	if json.Unmarshal([]byte(dbl), &m) != nil {
		t.Fatalf("double %s", dbl)
	}
}

func TestNewTenantSuperAdminStampsTimes(t *testing.T) {
	now := int64(1700000000)
	admin := newTenantSuperAdmin(0, 9, "pair9", "hash", now)
	if admin.ID != 0 || admin.TenantID != 9 || admin.Account != "pair9" || admin.Name != "超级管理员" {
		t.Fatalf("%+v", admin)
	}
	if admin.Root != 1 || admin.MultipointLogin != 1 || admin.CreateTime != now {
		t.Fatalf("fields %+v", admin)
	}
	if admin.UpdateTime == nil || *admin.UpdateTime != now {
		t.Fatalf("update_time %+v", admin.UpdateTime)
	}
	sharded := newTenantSuperAdmin(1, 9, "admin", "hash", now)
	if sharded.ID != 1 || sharded.UpdateTime == nil || *sharded.UpdateTime != now {
		t.Fatalf("sharded %+v", sharded)
	}
}

func TestTenantAliasTakenSkipsEmpty(t *testing.T) {
	if tenantAliasTaken("", 0) {
		t.Fatal("empty alias must skip uniqueness like PHP TenantValidate")
	}
	if tenantAliasTaken("", 7) {
		t.Fatal("empty alias with exclude must still skip")
	}
}

func TestRandomSNCharsetAndUniqueness(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		sn := randomSN()
		if !validTenantSN(sn) || len(sn) != 8 {
			t.Fatalf("sn %q", sn)
		}
		for _, r := range sn {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
				t.Fatalf("charset %q in %s", r, sn)
			}
		}
		if seen[sn] {
			t.Fatalf("duplicate %s", sn)
		}
		seen[sn] = true
	}
}

func TestTenantAddGeneratesSNWithoutHostName(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	name := fmt.Sprintf("autosn-%d", time.Now().UnixNano())
	account := fmt.Sprintf("a%d", time.Now().UnixNano()%1e9)
	db := bootstrap.DB
	cleanup := func() {
		var tenant model.Tenant
		if err := db.Where("name = ?", name).First(&tenant).Error; err != nil {
			return
		}
		tid := tenant.ID
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
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := fmt.Sprintf(`{"name":%q,"disable":0,"tactics":0,"domain_alias":"","domain_alias_enable":1,"account":%q,"password":"likeadmin","password_confirm":"likeadmin"}`, name, account)
	c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/tenant.tenant/add", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	TenantAdd(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var resp struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Code != 1 {
		t.Fatalf("add failed: %s", w.Body.String())
	}
	var tenant model.Tenant
	if err := db.Where("name = ? AND delete_time IS NULL", name).First(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	if !validTenantSN(tenant.SN) || len(tenant.SN) != 8 {
		t.Fatalf("generated sn %q", tenant.SN)
	}
}
