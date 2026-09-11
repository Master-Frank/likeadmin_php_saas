package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Data(c, map[string]any{"token": "abc"})
	var body Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != 1 || body.Show != 0 {
		t.Fatalf("code/show %+v", body)
	}
	m := body.Data.(map[string]any)
	if m["token"] != "abc" {
		t.Fatalf("data %+v", body.Data)
	}
}

func TestFailEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	Fail(c, "密码错误")
	var raw map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &raw)
	if raw["code"].(float64) != 0 {
		t.Fatalf("%v", raw)
	}
	if _, ok := raw["data"].([]any); !ok {
		t.Fatalf("data should be array, got %T", raw["data"])
	}
}

func TestDataCachedETag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := map[string]any{"ok": 1}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/index/config", nil)
	DataCached(c, payload, 0)
	etag := w.Header().Get("ETag")
	if etag == "" || w.Header().Get("Cache-Control") == "" {
		t.Fatalf("headers %+v", w.Header())
	}
	if w.Code != 200 {
		t.Fatalf("code %d", w.Code)
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/api/index/config", nil)
	c2.Request.Header.Set("If-None-Match", etag)
	DataCached(c2, payload, 0)
	if w2.Code != 304 {
		t.Fatalf("want 304 got %d body=%s etag=%q", w2.Code, w2.Body.String(), etag)
	}
}
