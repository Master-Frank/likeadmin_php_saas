package filesvc

import (
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
)

func TestUploadCateOKTenantIsolation(t *testing.T) {
	if bootstrap.DB == nil {
		cfg := os.Getenv("LIKEADMIN_CONFIG")
		if cfg == "" {
			cfg = "/workspace/backend/configs/config.yaml"
		}
		if err := bootstrap.Init(cfg); err != nil {
			t.Skip(err)
		}
	}
	if bootstrap.DB == nil {
		t.Skip("no database")
	}
	tenantdb.Register(bootstrap.DB)
	now := util.NowUnix()
	a := model.TenantFileCate{Name: "cate-990006", Type: 10, TenantID: 990006, CreateTime: now, UpdateTime: &now}
	b := model.TenantFileCate{Name: "cate-990007", Type: 10, TenantID: 990007, CreateTime: now, UpdateTime: &now}
	if err := bootstrap.DB.Create(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.DB.Create(&b).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = bootstrap.DB.Where("id IN ?", []uint{a.ID, b.ID}).Delete(&model.TenantFileCate{}).Error
	})
	if msg := UploadCateOK(bootstrap.DB, &model.TenantFileCate{}, a.ID, 990006); msg != "" {
		t.Fatalf("own cate: %s", msg)
	}
	if msg := UploadCateOK(bootstrap.DB, &model.TenantFileCate{}, a.ID, 990007); msg != "文件分类不存在" {
		t.Fatalf("cross-tenant cate: %s", msg)
	}
	if msg := UploadCateOK(bootstrap.DB, &model.TenantFileCate{}, 999999001, 990006); msg != "文件分类不存在" {
		t.Fatalf("missing cate: %s", msg)
	}
}
