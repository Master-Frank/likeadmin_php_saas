package upgrade

import (
	"os"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
)

func TestUpgradeMenusForDisposableTenant(t *testing.T) {
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if bootstrap.DB == nil {
		if err := bootstrap.Init(cfg); err != nil {
			t.Skip(err)
		}
	}
	if bootstrap.DB == nil {
		t.Skip("no database")
	}
	tenantdb.Register(bootstrap.DB)

	var tpls int64
	if err := bootstrap.DB.Model(&model.TenantSystemMenu{}).Where("tenant_id = 0").Count(&tpls).Error; err != nil || tpls == 0 {
		t.Skip("no tenant_id=0 menu templates")
	}

	now := util.NowUnix()
	row := model.Tenant{
		SN: "upgmenu61", Name: "upgrade-menu-smoke", Tactics: 0,
		CreateTime: now, UpdateTime: &now,
	}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = bootstrap.DB.Where("tenant_id = ?", row.ID).Delete(&model.TenantSystemMenu{}).Error
		_ = bootstrap.DB.Unscoped().Delete(&row).Error
	})
	junk := model.TenantSystemMenu{
		TenantID: row.ID, Name: "junk-should-be-wiped", Type: "C", Paths: "/junk",
		CreateTime: now, UpdateTime: &now,
	}
	if err := bootstrap.DB.Create(&junk).Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if _, err := os.Stat(dir); err != nil {
		t.Fatal(err)
	}
	// Missing dir is a no-op (PHP file_exists false).
	if err := upgradeMenu(bootstrap.DB, dir+"/missing"); err != nil {
		t.Fatal(err)
	}
	var stillJunk int64
	bootstrap.DB.Model(&model.TenantSystemMenu{}).Where("id = ?", junk.ID).Count(&stillJunk)
	if stillJunk != 1 {
		t.Fatal("missing menu dir must not wipe tenants")
	}

	if err := upgradeMenusFor(bootstrap.DB, []model.Tenant{row}); err != nil {
		t.Fatal(err)
	}
	var junkLeft int64
	bootstrap.DB.Model(&model.TenantSystemMenu{}).Where("id = ?", junk.ID).Count(&junkLeft)
	if junkLeft != 0 {
		t.Fatal("upgradeMenu must delete old tenant menus")
	}
	var got int64
	if err := bootstrap.DB.Model(&model.TenantSystemMenu{}).Where("tenant_id = ?", row.ID).Count(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got != tpls {
		t.Fatalf("reinit menus %d want templates %d", got, tpls)
	}

	var child model.TenantSystemMenu
	if err := bootstrap.DB.Where("tenant_id = ? AND pid > 0", row.ID).First(&child).Error; err != nil {
		t.Skip("no remapped child")
	}
	var parent model.TenantSystemMenu
	if err := bootstrap.DB.Where("id = ? AND tenant_id = ?", child.Pid, row.ID).First(&parent).Error; err != nil {
		t.Fatalf("pid %d not remapped onto tenant %d: %v", child.Pid, row.ID, err)
	}
	if child.UpdateTime == nil || *child.UpdateTime < now-2 {
		t.Fatalf("remap update_time=%v", child.UpdateTime)
	}
	if child.CreateTime < time.Now().Unix()-30 {
		t.Fatalf("create_time leaked template: %d", child.CreateTime)
	}

	var pair1 int64
	bootstrap.DB.Model(&model.TenantSystemMenu{}).Where("tenant_id = 1").Count(&pair1)
	if pair1 == 0 {
		t.Fatal("must not wipe pairing tenant 1 menus")
	}
}
