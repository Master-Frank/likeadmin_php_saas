package httpx

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestQueryIgnoresJSONBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/pay/payWay?from=query", bytes.NewBufferString(`{"from":"json","order_id":9}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if QueryStr(c, "from") != "query" {
		t.Fatalf("query from=%q", QueryStr(c, "from"))
	}
	if QueryStr(c, "order_id") != "" {
		t.Fatalf("body must not leak into Query, got %q", QueryStr(c, "order_id"))
	}
	if Str(c, "from") != "json" {
		t.Fatalf("Params should still merge body, got %q", Str(c, "from"))
	}
	if QueryInt(c, "order_id") != 0 {
		t.Fatalf("QueryInt must ignore JSON body")
	}
	if BodyStr(c, "from") != "json" || BodyUint(c, "order_id") != 9 {
		t.Fatalf("Body from=%q id=%d", BodyStr(c, "from"), BodyUint(c, "order_id"))
	}
	if BodyStr(c, "missing") != "" {
		t.Fatal("query must not leak into Body")
	}
	if BodyInt(c, "order_id") != 9 || BodyFloat(c, "order_id") != 9 {
		t.Fatalf("BodyInt/Float order_id=%d/%v", BodyInt(c, "order_id"), BodyFloat(c, "order_id"))
	}
	if BodyHas(c, "from") != true || BodyHas(c, "missing") {
		t.Fatal("BodyHas mismatch")
	}
}

func TestBodyIgnoresQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/pay/prepay?from=recharge&pay_way=2&order_id=1", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if BodyStr(c, "from") != "" || BodyHas(c, "from") || BodyUint(c, "order_id") != 0 {
		t.Fatalf("query leaked into Body from=%q id=%d has=%v", BodyStr(c, "from"), BodyUint(c, "order_id"), BodyHas(c, "from"))
	}
	if Str(c, "from") != "recharge" {
		t.Fatalf("Params still merges query, got %q", Str(c, "from"))
	}
}

func TestBodyIDPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := func(body string) bool {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/auth.role/delete", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return BodyIDPresent(c)
	}
	if req(`{}`) {
		t.Fatal("missing id must be absent")
	}
	if req(`{"id":""}`) {
		t.Fatal("empty id must be absent")
	}
	if !req(`{"id":0}`) {
		t.Fatal("id=0 is present under ThinkPHP require")
	}
	if !req(`{"id":"0"}`) {
		t.Fatal(`id:"0" is present`)
	}
	if !req(`{"id":12}`) {
		t.Fatal("id=12 must be present")
	}
}

func TestQueryIDPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := func(raw string) bool {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/auth.role/detail"+raw, nil)
		return QueryIDPresent(c)
	}
	if req("") {
		t.Fatal("missing id must be absent")
	}
	if req("?foo=1") {
		t.Fatal("other keys must not count")
	}
	if req("?id=") {
		t.Fatal("empty id must be absent")
	}
	if !req("?id=0") {
		t.Fatal("id=0 is present under ThinkPHP require")
	}
	if !req("?id=12") {
		t.Fatal("id=12 must be present")
	}
}

func TestParamTenantID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := func(raw, body string) (uint, bool) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/x"+raw, bytes.NewBufferString(body))
		if body != "" {
			c.Request.Header.Set("Content-Type", "application/json")
		}
		return ParamTenantID(c)
	}
	if id, ok := req("", `{}`); ok || id != 0 {
		t.Fatal("missing")
	}
	if id, ok := req("", `{"tenant_id":2}`); !ok || id != 2 {
		t.Fatalf("body %d %v", id, ok)
	}
	if id, ok := req("", `{"tenantId":7}`); !ok || id != 7 {
		t.Fatalf("tenantId %d %v", id, ok)
	}
	if id, ok := req("?tenant_id=2", `{"tenant_id":9}`); !ok || id != 9 {
		t.Fatalf("body wins %d %v", id, ok)
	}
	if id, ok := req("?tenant_id=3", `{}`); !ok || id != 3 {
		t.Fatalf("query %d %v", id, ok)
	}
}
