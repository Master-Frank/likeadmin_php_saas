package tenantapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func TestTenantUploadRejectsForeignCate(t *testing.T) {
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
	cate := model.TenantFileCate{Name: "foreign-990006", Type: 10, TenantID: 990006, CreateTime: now, UpdateTime: &now}
	if err := bootstrap.DB.Create(&cate).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = bootstrap.DB.Where("id = ?", cate.ID).Delete(&model.TenantFileCate{}).Error
	})

	gin.SetMode(gin.TestMode)
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("cid", util.ToString(cate.ID))
	_ = mw.Close()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/tenantapi/upload/image", &body)
	c.Request.Header.Set("Content-Type", mw.FormDataContentType())
	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant, TenantID: 990007})

	UploadImage(c)
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code == 1 || wrap.Msg != "文件分类不存在" {
		t.Fatalf("foreign cid: %+v body=%s", wrap, w.Body.String())
	}
}
