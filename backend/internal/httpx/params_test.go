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
