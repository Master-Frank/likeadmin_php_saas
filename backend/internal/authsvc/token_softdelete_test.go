package authsvc

import (
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
)

func initAuthDB(t *testing.T) bool {
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
	return bootstrap.DB != nil
}

func TestExpirePlatformTokenDeletedAdmin(t *testing.T) {
	if !initAuthDB(t) {
		t.Skip("no database")
	}
	var tmpl model.Admin
	if bootstrap.DB.Where("delete_time IS NULL").First(&tmpl).Error != nil {
		t.Skip("no admin")
	}
	now := util.NowUnix()
	tmpl.ID = 0
	tmpl.Account = "pair-soft-adm-" + util.ToString(now)
	tmpl.Name = "pair-soft-adm"
	tmpl.MultipointLogin = 1
	tmpl.Root = 0
	tmpl.CreateTime = now
	tmpl.UpdateTime = util.UnixPtr(now)
	tmpl.DeleteTime = util.UnixPtr(now)
	if err := bootstrap.DB.Create(&tmpl).Error; err != nil {
		t.Fatal(err)
	}
	token := "pair-soft-tok-" + util.ToString(now)
	sess := model.AdminSession{AdminID: tmpl.ID, Terminal: 1, Token: token, ExpireTime: now + 3600, UpdateTime: util.UnixPtr(now)}
	if err := bootstrap.DB.Create(&sess).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("id = ?", sess.ID).Delete(&model.AdminSession{})
		bootstrap.DB.Where("id = ?", tmpl.ID).Delete(&model.Admin{})
	})
	if !ExpirePlatformToken(token) {
		t.Fatal("PHP expireToken treats SoftDelete admin as empty and still expires")
	}
	var got model.AdminSession
	bootstrap.DB.Where("id = ?", sess.ID).First(&got)
	if got.ExpireTime > now {
		t.Fatalf("session should expire, expire_time=%d now=%d", got.ExpireTime, now)
	}
}
