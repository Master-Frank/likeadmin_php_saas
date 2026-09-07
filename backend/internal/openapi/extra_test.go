package openapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"

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
	info := wechatUserInfo(c, model.User{
		ID: 9, SN: 1001, Mobile: "13800000000", Nickname: "n",
		Avatar: "uploads/a.png", IsDisable: 0, IsNewUser: 1,
	}, "tok")
	for _, key := range []string{"id", "sn", "mobile", "nickname", "avatar", "is_disable", "is_new_user", "token"} {
		if _, ok := info[key]; !ok {
			t.Fatalf("missing %s in %v", key, info)
		}
	}
	if info["is_disable"] != 0 || info["token"] != "tok" || info["id"] != uint(9) {
		t.Fatalf("%v", info)
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
