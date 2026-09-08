package tenantapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func TestLoginAccountWritesShardedAdminSession(t *testing.T) {
	if !initAdjustShardDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990021
	const sn = "t990021"
	const adminID uint = 99002101
	const account = "a990021"
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Exec("DROP TABLE IF EXISTS la_tenant_admin_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_tenant_admin_session_" + sn).Error
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().Unix()
	if err := db.Create(&model.Tenant{
		ID: tid, SN: sn, Name: "login-shard", Tactics: 1, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"la_tenant_admin", "la_tenant_admin_session"} {
		if err := db.Exec("CREATE TABLE " + name + "_" + sn + " LIKE " + name).Error; err != nil {
			t.Fatal(err)
		}
	}
	sdb := tenantdb.UseSN(sn)
	if err := sdb.Create(&model.TenantAdmin{
		ID: adminID, TenantID: tid, Account: account, Name: "分表登录", Root: 1,
		Password: util.CreatePassword("likeadmin1", config.C.Project.UniqueIdentification),
		MultipointLogin: 1, LoginTime: util.UnixPtr(now), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(map[string]any{
		"account": account, "password": "likeadmin1", "terminal": 1,
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/tenantapi/login/account", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Host = "t990021.likeadmin.test"
	ctxutil.Set(c, &ctxutil.RequestMeta{
		Source: ctxutil.SourceTenant, TenantID: tid, TenantSN: sn, Tactics: 1, App: "tenantapi",
	})
	LoginAccount(c)
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code != 1 {
		t.Fatalf("login %+v body=%s", wrap, w.Body.String())
	}
	data, _ := wrap.Data.(map[string]any)
	token := util.ToString(data["token"])
	if token == "" {
		t.Fatalf("token missing %+v", wrap.Data)
	}

	var shardSess model.TenantAdminSession
	if err := sdb.Where("token = ?", token).First(&shardSess).Error; err != nil {
		t.Fatalf("shard session: %v", err)
	}
	if shardSess.AdminID != adminID {
		t.Fatalf("session %+v", shardSess)
	}
	var shared int64
	db.Model(&model.TenantAdminSession{}).Where("token = ?", token).Count(&shared)
	if shared != 0 {
		t.Fatalf("shared session count=%d", shared)
	}
}
