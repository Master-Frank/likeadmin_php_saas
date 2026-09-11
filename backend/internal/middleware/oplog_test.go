package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"

	"github.com/gin-gonic/gin"
)

func TestRedactParamsNestedCredentials(t *testing.T) {
	in := map[string]any{
		"name": "ok",
		"config": map[string]any{
			"private_key":       "pk",
			"mch_key":           "mk",
			"access_key_secret": "aks",
			"domain":            "cdn.example",
		},
		"list": []any{
			map[string]any{"app_secret": "s", "pay_way": 1},
		},
		"password": "plain",
	}
	got, ok := redactParams(in).(map[string]any)
	if !ok {
		t.Fatalf("%T", redactParams(in))
	}
	if got["name"] != "ok" || got["password"] != "******" {
		t.Fatalf("top %#v", got)
	}
	cfg := got["config"].(map[string]any)
	if cfg["private_key"] != "******" || cfg["mch_key"] != "******" || cfg["access_key_secret"] != "******" {
		t.Fatalf("nested %#v", cfg)
	}
	if cfg["domain"] != "cdn.example" {
		t.Fatal("non-secret nested field redacted")
	}
	item := got["list"].([]any)[0].(map[string]any)
	if item["app_secret"] != "******" || item["pay_way"] != 1 {
		t.Fatalf("list %#v", item)
	}
}

func TestCappedBodyWriter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	bw := &bodyWriter{ResponseWriter: c.Writer, cap: 8}
	n, err := bw.Write([]byte("hello world"))
	if err != nil || n != 11 {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	if bw.buf.Len() != 8 || !bw.truncated {
		t.Fatalf("cap buf=%d truncated=%v", bw.buf.Len(), bw.truncated)
	}
	if w.Body.String() != "hello world" {
		t.Fatalf("downstream %q", w.Body.String())
	}
	n, _ = bw.Write([]byte("more"))
	if n != 4 || bw.buf.Len() != 8 {
		t.Fatalf("second write grew buffer to %d", bw.buf.Len())
	}
}

func TestSkipOplogCapture(t *testing.T) {
	gin.SetMode(gin.TestMode)
	get := httptest.NewRequest(http.MethodGet, "/platformapi/auth.admin/lists", nil)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = get
	if !skipOplogCapture(c, &ctxutil.RequestMeta{Controller: "auth.admin"}) {
		t.Fatal("GET must skip capture")
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/tenantapi/download/export", nil)
	if !skipOplogCapture(c, &ctxutil.RequestMeta{Controller: "download", Action: "export"}) {
		t.Fatal("download must skip capture")
	}
	c.Request = httptest.NewRequest(http.MethodPost, "/platformapi/auth.admin/add", nil)
	if skipOplogCapture(c, &ctxutil.RequestMeta{Controller: "auth.admin", Action: "add"}) {
		t.Fatal("POST writes still capture")
	}
}

func TestOplogMustPersistWrites(t *testing.T) {
	if !oplogMustPersist(model.OperationLog{Type: "POST", URL: "/platformapi/auth.admin/add"}) {
		t.Fatal("POST must persist")
	}
	if oplogMustPersist(model.OperationLog{Type: "GET", URL: "/platformapi/auth.admin/lists"}) {
		t.Fatal("GET lists may drop")
	}
	if !oplogMustPersist(model.OperationLog{Type: "GET", URL: "/platformapi/login/account"}) {
		t.Fatal("login URL must persist")
	}
}
