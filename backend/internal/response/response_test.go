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
