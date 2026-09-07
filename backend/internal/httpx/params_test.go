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
}
