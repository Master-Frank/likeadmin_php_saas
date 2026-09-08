package openapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

func TestAuthWechatUserWritesShardedUserAuthAndSession(t *testing.T) {
	if !initOpenapiDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990020
	const sn = "t990020"
	const openid = "go-wx-990020"
	db := bootstrap.DB
	tenantdb.Register(db)
	cleanup := func() {
		_ = db.Exec("DROP TABLE IF EXISTS la_user_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_user_auth_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_user_session_" + sn).Error
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
		_ = db.Where("openid = ?", openid).Delete(&model.UserAuth{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().Unix()
	if err := db.Create(&model.Tenant{
		ID: tid, SN: sn, Name: "wx-shard", Tactics: 1, CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"la_user", "la_user_auth", "la_user_session"} {
		if err := db.Exec("CREATE TABLE " + name + "_" + sn + " LIKE " + name).Error; err != nil {
			t.Fatal(err)
		}
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login/mnpLogin", nil)
	c.Request.Host = "t990020.likeadmin.test"
	ctxutil.Set(c, &ctxutil.RequestMeta{
		Source: ctxutil.SourceUser, TenantID: tid, TenantSN: sn, Tactics: 1, App: "api",
	})
	info, err := authWechatUser(c, wechat.Session{Openid: openid, Nickname: "wx990020"}, wechat.TerminalMNP, true)
	if err != nil {
		t.Fatal(err)
	}
	if util.ToString(info["token"]) == "" || util.ToInt(info["sn"]) == 0 {
		t.Fatalf("info %+v", info)
	}

	sdb := tenantdb.UseSN(sn)
	var shardUser model.User
	uid := uint(util.ToInt(info["id"]))
	if err := sdb.Where("id = ? AND delete_time IS NULL", uid).First(&shardUser).Error; err != nil {
		t.Fatalf("shard user: %v", err)
	}
	var shardAuth model.UserAuth
	if err := sdb.Where("openid = ?", openid).First(&shardAuth).Error; err != nil {
		t.Fatalf("shard auth: %v", err)
	}
	if shardAuth.UserID != shardUser.ID || shardAuth.TenantID != tid {
		t.Fatalf("auth %+v user %+v", shardAuth, shardUser)
	}
	var shardSess model.UserSession
	if err := sdb.Where("user_id = ?", shardUser.ID).First(&shardSess).Error; err != nil {
		t.Fatalf("shard session: %v", err)
	}
	var sharedUsers, sharedAuth, sharedSess int64
	db.Model(&model.User{}).Where("account = ?", shardUser.Account).Count(&sharedUsers)
	db.Model(&model.UserAuth{}).Where("openid = ?", openid).Count(&sharedAuth)
	db.Model(&model.UserSession{}).Where("token = ?", shardSess.Token).Count(&sharedSess)
	if sharedUsers != 0 || sharedAuth != 0 || sharedSess != 0 {
		t.Fatalf("shared leak users=%d auth=%d sess=%d", sharedUsers, sharedAuth, sharedSess)
	}
}
