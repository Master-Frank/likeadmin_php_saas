package tenantapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
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

func initAdjustShardDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
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

func TestUserAdjustMoneyWritesShardedUserAndLog(t *testing.T) {
	if !initAdjustShardDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990018
	const sn = "t990018"
	const uid uint = 99001801
	db := bootstrap.DB
	cleanup := func() {
		_ = db.Exec("DROP TABLE IF EXISTS la_user_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_user_account_log_" + sn).Error
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
		_ = db.Where("id = ?", uid).Delete(&model.User{}).Error
		_ = db.Where("user_id = ?", uid).Delete(&model.UserAccountLog{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().Unix()
	if err := db.Create(&model.Tenant{
		ID: tid, SN: sn, Name: "adjust-shard", Tactics: 1, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"la_user", "la_user_account_log"} {
		if err := db.Exec("CREATE TABLE " + name + "_" + sn + " LIKE " + name).Error; err != nil {
			t.Fatal(err)
		}
	}
	sdb := tenantdb.UseSN(sn)
	if err := sdb.Create(&model.User{
		ID: uid, TenantID: tid, Account: "adj990018", Nickname: "adj", SN: 990018,
		UserMoney: 10, LoginTime: util.UnixPtr(now), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	decoy := model.User{
		ID: uid, TenantID: 1, Account: "adj990018-shared", Nickname: "decoy", SN: 880018,
		UserMoney: 99, LoginTime: util.UnixPtr(now), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if err := db.Create(&decoy).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body, _ := json.Marshal(map[string]any{
		"user_id": uid, "action": 1, "num": 5, "remark": "go-adjust-990018",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/tenantapi/user.user/adjustmoney", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	ctxutil.Set(c, &ctxutil.RequestMeta{
		Source: ctxutil.SourceTenant, TenantID: tid, TenantSN: sn, Tactics: 1, App: "tenantapi",
	})
	UserAdjustMoney(c)
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code != 1 {
		t.Fatalf("adjust %+v body=%s", wrap, w.Body.String())
	}

	var shardUser model.User
	if err := sdb.Where("id = ?", uid).First(&shardUser).Error; err != nil {
		t.Fatal(err)
	}
	if shardUser.UserMoney != 15 {
		t.Fatalf("shard money=%v want 15", shardUser.UserMoney)
	}
	var sharedUser model.User
	if err := db.Where("id = ?", uid).First(&sharedUser).Error; err != nil {
		t.Fatal(err)
	}
	if sharedUser.UserMoney != 99 {
		t.Fatalf("shared decoy money=%v want 99", sharedUser.UserMoney)
	}

	var shardLogs int64
	sdb.Model(&model.UserAccountLog{}).Where("user_id = ? AND remark = ?", uid, "go-adjust-990018").Count(&shardLogs)
	if shardLogs != 1 {
		t.Fatalf("shard logs=%d", shardLogs)
	}
	var sharedLogs int64
	db.Model(&model.UserAccountLog{}).Where("user_id = ? AND remark = ?", uid, "go-adjust-990018").Count(&sharedLogs)
	if sharedLogs != 0 {
		t.Fatalf("shared logs=%d (must stay on shard)", sharedLogs)
	}
}
