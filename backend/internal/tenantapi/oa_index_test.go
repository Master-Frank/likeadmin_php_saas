package tenantapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

func initOAIndexDB(t *testing.T) bool {
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
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
	}
	return bootstrap.DB != nil
}

func TestOAReplyIndexEchostrAndKeyword(t *testing.T) {
	if !initOAIndexDB(t) {
		t.Skip("no database")
	}
	const tid uint = 990015
	db := bootstrap.DB
	now := time.Now().Unix()
	row := model.OfficialAccountReply{
		TenantID: tid, Name: "go-oa-990015", Keyword: "你好", ReplyType: wechat.ReplyKeyword,
		MatchingType: wechat.MatchFull, ContentType: 1, Content: "欢迎光临", Status: 1, Sort: 1,
		CreateTime: now, UpdateTime: util.UnixPtr(now),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = db.Where("tenant_id = ?", tid).Delete(&model.OfficialAccountReply{}).Error
	})

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenantapi/channel.official_account_reply/index?echostr=ping990015", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant, TenantID: tid, App: "tenantapi"})
	OAReplyIndex(c)
	if w.Body.String() != "ping990015" {
		t.Fatalf("echostr %q", w.Body.String())
	}

	xmlBody := `<xml><ToUserName><![CDATA[gh_go]]></ToUserName><FromUserName><![CDATA[ou_go]]></FromUserName><CreateTime>1757325600</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[你好]]></Content></xml>`
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodPost, "/tenantapi/channel.official_account_reply/index", strings.NewReader(xmlBody))
	c2.Request.Header.Set("Content-Type", "text/xml")
	ctxutil.Set(c2, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant, TenantID: tid, App: "tenantapi"})
	OAReplyIndex(c2)
	got := w2.Body.String()
	if !strings.Contains(got, "欢迎光临") || !strings.Contains(got, "<MsgType><![CDATA[text]]></MsgType>") {
		t.Fatalf("keyword reply %q", got)
	}
	if !strings.Contains(got, "<ToUserName><![CDATA[ou_go]]></ToUserName>") {
		t.Fatalf("reply to user %q", got)
	}
}
