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

func TestQueryRawKeepsSpaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/tenantapi/dept.dept/lists?name=%20%20&status=0", nil)
	if QueryRaw(c, "name") != "  " || QueryStr(c, "name") != "" {
		t.Fatalf("raw=%q trim=%q", QueryRaw(c, "name"), QueryStr(c, "name"))
	}
	if QueryRaw(c, "status") != "0" {
		t.Fatalf("status=%q", QueryRaw(c, "status"))
	}
}

func TestBodyUintsUnlessEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := func(body string) []uint {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/auth.role/add", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return BodyUintsUnlessEmpty(c, "menu_id")
	}
	if ids := req(`{"menu_id":"0"}`); len(ids) != 0 {
		t.Fatalf("scalar 0 must be empty, got %v", ids)
	}
	if ids := req(`{"menu_id":0}`); len(ids) != 0 {
		t.Fatalf("int 0 must be empty, got %v", ids)
	}
	if ids := req(`{"menu_id":[]}`); len(ids) != 0 {
		t.Fatalf("[] must be empty, got %v", ids)
	}
	if ids := req(`{"menu_id":[0]}`); len(ids) != 1 || ids[0] != 0 {
		t.Fatalf("[0] is not empty(), got %v", ids)
	}
	if ids := req(`{"menu_id":[1,2]}`); len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("ids=%v", ids)
	}
}

func TestBodyRawKeepsSpaces(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/login/register", bytes.NewBufferString(`{"account":" ab12 ","mobile":" 13800138000 "}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if BodyRaw(c, "account") != " ab12 " || BodyStr(c, "account") != "ab12" {
		t.Fatalf("raw=%q trim=%q", BodyRaw(c, "account"), BodyStr(c, "account"))
	}
	if BodyRaw(c, "mobile") != " 13800138000 " {
		t.Fatalf("mobile raw=%q", BodyRaw(c, "mobile"))
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
	if !req(`{"id":"   "}`) {
		t.Fatal("whitespace id passes ThinkPHP require")
	}
	if req(`{"id":null}`) {
		t.Fatal("JSON null id is absent")
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
