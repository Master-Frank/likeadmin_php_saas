package biz

import (
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
)

func initAccountDB(t *testing.T) bool {
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

func TestAddAccountLogRejectsUnknownType(t *testing.T) {
	if !initAccountDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("delete_time IS NULL").First(&user).Error != nil {
		t.Skip("no user")
	}
	var before int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("user_id = ?", user.ID).Count(&before)
	AddAccountLog(bootstrap.DB, user.ID, user.TenantID, 999, INC, 1, user.UserMoney, "", "bogus")
	var after int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("user_id = ?", user.ID).Count(&after)
	if after != before {
		t.Fatal("PHP AccountLogLogic::add rejects unknown change_type")
	}
}

func TestAddAccountLogRejectsMissingUser(t *testing.T) {
	if !initAccountDB(t) {
		t.Skip("no database")
	}
	var before int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("user_id = 0").Count(&before)
	AddAccountLog(bootstrap.DB, 0, 1, UMIncAdmin, INC, 1, 0, "", "missing")
	var after int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("user_id = 0").Count(&after)
	if after != before {
		t.Fatal("PHP AccountLogLogic::add rejects missing user")
	}
}

func TestAddAccountLogWritesKnownType(t *testing.T) {
	if !initAccountDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("delete_time IS NULL").First(&user).Error != nil {
		t.Skip("no user")
	}
	AddAccountLog(bootstrap.DB, user.ID, user.TenantID, UMIncAdmin, INC, 0.01, user.UserMoney, "", "pair-account-61")
	var row model.UserAccountLog
	if bootstrap.DB.Where("user_id = ? AND remark = ?", user.ID, "pair-account-61").Order("id desc").First(&row).Error != nil {
		t.Fatal("known change_type should write")
	}
	t.Cleanup(func() { bootstrap.DB.Delete(&row) })
	if row.ChangeType != UMIncAdmin || row.ChangeObject != UM {
		t.Fatalf("type=%d object=%d", row.ChangeType, row.ChangeObject)
	}
}
