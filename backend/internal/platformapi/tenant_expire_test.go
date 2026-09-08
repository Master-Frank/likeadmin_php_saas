package platformapi

import (
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
)

func TestExpireTenantAdminsKicksShardedUserSessions(t *testing.T) {
	if !initTenantCloneDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990011
	const sn = "t990011"
	const uid uint = 99001101
	const adminID uint = 99001102
	db := bootstrap.DB
	tenantdb.Register(db)
	cleanup := func() {
		_ = db.Exec("DROP TABLE IF EXISTS la_user_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_user_session_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_tenant_admin_" + sn).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_tenant_admin_session_" + sn).Error
		_ = db.Where("id = ?", tid).Delete(&model.Tenant{}).Error
		_ = db.Where("token LIKE ?", "go-expire-990011-%").Delete(&model.UserSession{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	for _, name := range []string{"la_user", "la_user_session", "la_tenant_admin", "la_tenant_admin_session"} {
		if err := db.Exec("CREATE TABLE " + name + "_" + sn + " LIKE " + name).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().Unix()
	tenant := model.Tenant{ID: tid, SN: sn, Name: "expire-shard", Tactics: 1, CreateTime: now, UpdateTime: util.UnixPtr(now)}
	if err := db.Create(&tenant).Error; err != nil {
		t.Fatal(err)
	}
	sdb := tenantdb.UseSN(sn)
	if err := sdb.Create(&model.TenantAdmin{
		ID: adminID, TenantID: tid, Account: "exp-admin", Name: "禁用踢会话", Root: 1,
		CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := sdb.Create(&model.TenantAdminSession{
		AdminID: adminID, Terminal: 1, Token: "go-expire-990011-admin", ExpireTime: now + 3600,
		UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := sdb.Create(&model.User{
		ID: uid, TenantID: tid, Account: "exp-user", Nickname: "exp-user", SN: 990011,
		LoginTime: util.UnixPtr(now), CreateTime: now, UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := sdb.Create(&model.UserSession{
		TenantID: tid, UserID: uid, Terminal: 1, Token: "go-expire-990011-user", ExpireTime: now + 3600,
		UpdateTime: util.UnixPtr(now),
	}).Error; err != nil {
		t.Fatal(err)
	}
	decoy := model.UserSession{
		TenantID: 1, UserID: uid, Terminal: 1, Token: "go-expire-990011-shared", ExpireTime: now + 7200,
		UpdateTime: util.UnixPtr(now),
	}
	if err := db.Create(&decoy).Error; err != nil {
		t.Fatal(err)
	}

	expireTenantAdmins(tenant)

	var adminSess model.TenantAdminSession
	if err := sdb.Where("token = ?", "go-expire-990011-admin").First(&adminSess).Error; err != nil {
		t.Fatal(err)
	}
	if adminSess.ExpireTime >= now+3600 {
		t.Fatalf("shard admin session still live expire=%d", adminSess.ExpireTime)
	}
	var userSess model.UserSession
	if err := sdb.Where("token = ?", "go-expire-990011-user").First(&userSess).Error; err != nil {
		t.Fatal(err)
	}
	if userSess.ExpireTime >= now+3600 {
		t.Fatalf("shard user session still live expire=%d", userSess.ExpireTime)
	}
	var shared model.UserSession
	if err := db.Where("token = ?", "go-expire-990011-shared").First(&shared).Error; err != nil {
		t.Fatal(err)
	}
	if shared.ExpireTime < now+7200 {
		t.Fatalf("shared decoy session was kicked expire=%d", shared.ExpireTime)
	}
}
