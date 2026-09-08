package openapi

import (
	"os"
	"testing"

	"net/http/httptest"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func initOpenapiDB(t *testing.T) bool {
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

func TestUserSNTakenIgnoresSoftDeleted(t *testing.T) {
	if !initOpenapiDB(t) {
		t.Skip("no database")
	}
	var row model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL").First(&row).Error != nil {
		t.Skip("no tenant user")
	}
	sn := 80000000 + int(util.NowUnix()%9999999)
	now := util.NowUnix()
	row.ID = 0
	row.SN = sn
	row.Account = "pair-soft-sn-" + util.ToString(sn)
	row.Mobile = ""
	row.CreateTime = now
	row.UpdateTime = util.UnixPtr(now)
	row.DeleteTime = util.UnixPtr(now)
	if err := bootstrap.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bootstrap.DB.Where("id = ?", row.ID).Delete(&model.User{}) })

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctxutil.Set(c, &ctxutil.RequestMeta{TenantID: 1})
	if userSNTaken(c, bootstrap.DB, sn) {
		t.Fatal("PHP User::createUserSn SoftDelete find() treats deleted SN as free")
	}
}
