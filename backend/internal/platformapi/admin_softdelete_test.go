package platformapi

import (
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
)

func initAdminDB(t *testing.T) bool {
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

func TestRoleNamesSkipsSoftDeleted(t *testing.T) {
	if !initAdminDB(t) {
		t.Skip("no database")
	}
	now := util.NowUnix()
	row := model.SystemRole{Name: "pair-soft-role", Sort: 0, CreateTime: now, UpdateTime: util.UnixPtr(now), DeleteTime: util.UnixPtr(now)}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.SystemRole{}) })
	if names := roleNames([]uint{row.ID}); len(names) != 0 {
		t.Fatalf("PHP SystemRole SoftDelete must hide deleted role names, got %v", names)
	}
}

func TestDeptNamesSkipsSoftDeleted(t *testing.T) {
	if !initAdminDB(t) {
		t.Skip("no database")
	}
	now := util.NowUnix()
	row := model.Dept{Name: "pair-soft-dept", Pid: 0, Sort: 0, Status: 1, CreateTime: now, UpdateTime: util.UnixPtr(now), DeleteTime: util.UnixPtr(now)}
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.Dept{}) })
	if names := deptNames([]uint{row.ID}); len(names) != 0 {
		t.Fatalf("PHP Dept SoftDelete must hide deleted dept names, got %v", names)
	}
}

func TestDeptExistsAllowsRoot(t *testing.T) {
	if !deptExists(0) {
		t.Fatal("pid=0 is the virtual root; do not copy PHP checkDept reject")
	}
}
