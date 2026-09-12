package openapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

func TestUserSNTakenNilDB(t *testing.T) {
	if userSNTaken(nil, nil, 123) {
		t.Fatal("nil db")
	}
}

func TestDecoratePageValueEmpty(t *testing.T) {
	v := decoratePageValue(model.DecoratePage{})
	arr, ok := v.([]any)
	if !ok || len(arr) != 0 {
		t.Fatalf("empty page should be [], got %T %#v", v, v)
	}
}

func TestDecoratePageValuePresent(t *testing.T) {
	page := decoratePageMap(model.DecoratePage{ID: 3, Type: 1, Name: "首页"})
	if page["id"] != uint(3) || page["name"] != "首页" {
		t.Fatalf("%+v", page)
	}
	v := decoratePageValue(model.DecoratePage{ID: 3, Type: 1, Name: "首页"})
	m, ok := v.(gin.H)
	if !ok {
		t.Fatalf("present page should be object, got %T", v)
	}
	if m["name"] != "首页" {
		t.Fatalf("%+v", m)
	}
}

func TestArticleDetailMapFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	out := articleDetailMap(c, model.Article{
		ID: 9, Cid: 2, Title: "t", Desc: "d", Abstract: "a", Author: "au",
		IsShow: 1, Sort: 3, TenantID: 1, Content: "c",
	}, 11)
	for _, k := range []string{"id", "cid", "title", "desc", "abstract", "image", "author", "content", "is_show", "sort", "tenant_id", "click", "create_time", "update_time", "delete_time"} {
		if _, ok := out[k]; !ok {
			t.Fatalf("missing %s in %+v", k, out)
		}
	}
	if out["click"] != 11 || out["is_show"] != 1 || out["tenant_id"] != uint(1) {
		t.Fatalf("%+v", out)
	}
	if out["image"] != "" {
		t.Fatalf("empty image must stay empty, got %q", out["image"])
	}
}

func TestLoadVisibleArticleResolvesTemplateID(t *testing.T) {
	if !initOpenapiDB(t) {
		t.Skip("no database")
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/article/detail?id=3", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{TenantID: 1})
	a, ok := loadVisibleArticle(c, 3)
	if !ok {
		t.Fatal("template article 3 should resolve to tenant copy")
	}
	if a.TenantID != 1 {
		t.Fatalf("tenant_id=%d", a.TenantID)
	}
	if a.Title == "" || a.ID == 3 {
		t.Fatalf("expected tenant copy, got id=%d title=%q", a.ID, a.Title)
	}
}

func TestTenantArticleIDMapRewritesDecorateJSON(t *testing.T) {
	if !initOpenapiDB(t) {
		t.Skip("no database")
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{TenantID: 1})
	idMap := tenantArticleIDMap(c)
	if idMap[3] == 0 || idMap[3] == 3 {
		t.Fatalf("template id 3 should map onto tenant copy, got %v", idMap)
	}
	if idMap[6] != 0 && idMap[6] != idMap[3] {
		t.Fatalf("demo picker id 6 should alias onto the same copy as template 3, got %v", idMap)
	}
	raw := `[{"name":"banner","content":{"data":[{"link":{"path":"/pages/news_detail/news_detail","query":{"id":3},"type":"article","id":3}}]}}]`
	got := applyTenantArticleIDs(c, raw)
	if strings.Contains(got, `"id":3,`) || strings.Contains(got, `"id":3}`) {
		t.Fatalf("template id leaked: %s", got)
	}
	if !strings.Contains(got, fmt.Sprintf(`"id":%d`, idMap[3])) {
		t.Fatalf("expected tenant id %d in %s", idMap[3], got)
	}
}

func TestPcArticleMissingShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	out := pcArticleMissing(c, 0)
	if _, ok := out["last"].(map[string]any); !ok {
		t.Fatalf("last %+v", out["last"])
	}
	if _, ok := out["next"].(map[string]any); !ok {
		t.Fatalf("next %+v", out["next"])
	}
	if out["collect"] != false {
		t.Fatalf("collect %+v", out["collect"])
	}
	if out["cate_name"] != nil {
		t.Fatalf("cate_name %+v", out["cate_name"])
	}
	if _, ok := out["new"].([]map[string]any); !ok {
		t.Fatalf("new %+v", out["new"])
	}
}

func TestUserTerminalFromToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	if userTerminal(c) != 0 {
		t.Fatalf("empty token terminal=%d", userTerminal(c))
	}
	ctxutil.Set(c, &ctxutil.RequestMeta{UserInfo: map[string]any{"terminal": 2}})
	if userTerminal(c) != 2 {
		t.Fatalf("token terminal=%d", userTerminal(c))
	}
}

func TestWechatUserInfoFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	created := wechatUserInfo(c, model.User{
		ID: 9, SN: 1001, Account: "u1001", Mobile: "13800000000", Nickname: "n",
		Avatar: "uploads/a.png", Channel: 2, IsDisable: 0, IsNewUser: 1,
	}, "tok", true)
	for _, key := range []string{"id", "sn", "account", "channel", "mobile", "nickname", "avatar", "is_disable", "is_new_user", "token"} {
		if _, ok := created[key]; !ok {
			t.Fatalf("missing %s in %v", key, created)
		}
	}
	if created["is_disable"] != 0 || created["token"] != "tok" || created["id"] != uint(9) || created["account"] != "u1001" || created["channel"] != 2 {
		t.Fatalf("%v", created)
	}
	existing := wechatUserInfo(c, model.User{
		ID: 9, SN: 1001, Account: "u1001", Mobile: "13800000000", Nickname: "n",
		Avatar: "uploads/a.png", Channel: 2,
	}, "tok", false)
	if _, ok := existing["account"]; ok {
		t.Fatalf("existing wechat user must omit account: %v", existing)
	}
	if _, ok := existing["channel"]; ok {
		t.Fatalf("existing wechat user must omit channel: %v", existing)
	}
	empty := wechatUserInfo(c, model.User{ID: 1, Avatar: ""}, "t", false)
	if empty["avatar"] != "" {
		t.Fatalf("empty avatar should stay empty, got %v", empty["avatar"])
	}
}

func TestScanLoginAuthErr(t *testing.T) {
	if scanLoginAuthErr(wechat.Session{}) != "获取用户授权信息失败" {
		t.Fatal("empty session")
	}
	if scanLoginAuthErr(wechat.Session{Openid: "oid"}) != "获取用户授权信息失败" {
		t.Fatal("openid without access_token")
	}
	if scanLoginAuthErr(wechat.Session{AccessToken: "tok"}) != "获取用户授权信息失败" {
		t.Fatal("access_token without openid")
	}
	if scanLoginAuthErr(wechat.Session{Openid: "oid", AccessToken: "tok"}) != "" {
		t.Fatal("both present should pass")
	}
}

func TestUserCollectsArticleEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	if userCollectsArticle(c, 0, 1) || userCollectsArticle(c, 1, 0) {
		t.Fatal("empty ids should not collect")
	}
}
