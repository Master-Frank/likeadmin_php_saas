package openapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func TestLoginRegisterWritesShardedUser(t *testing.T) {
	if !initOpenapiDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990019
	const sn = "t990019"
	const account = "u990019a"
	db := bootstrap.DB
	tenantdb.Register(db)
	cleanup := func() {
		_ = db.Exec("DROP TABLE IF EXISTS la_user_" + sn).Error
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
		_ = db.Where("account = ?", account).Delete(&model.User{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().Unix()
	if err := db.Create(&model.Tenant{
		ID: tid, SN: sn, Name: "reg-shard", Tactics: 1, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE la_user_" + sn + " LIKE la_user").Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(map[string]any{
		"channel": 1, "account": account, "password": "Abcd12", "password_confirm": "Abcd12",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login/register", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ctxutil.Set(c, &ctxutil.RequestMeta{
		Source: ctxutil.SourceUser, TenantID: tid, TenantSN: sn, Tactics: 1, App: "api",
	})
	LoginRegister(c)
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code != 1 || wrap.Msg != "注册成功" {
		t.Fatalf("register %+v body=%s", wrap, w.Body.String())
	}

	sdb := tenantdb.UseSN(sn)
	var shardUser model.User
	if err := sdb.Where("account = ? AND delete_time IS NULL", account).First(&shardUser).Error; err != nil {
		t.Fatalf("shard user: %v", err)
	}
	if shardUser.TenantID != tid || shardUser.SN == 0 {
		t.Fatalf("shard user %+v", shardUser)
	}
	var shared int64
	db.Model(&model.User{}).Where("account = ?", account).Count(&shared)
	if shared != 0 {
		t.Fatalf("shared user count=%d (must stay on shard)", shared)
	}
}
