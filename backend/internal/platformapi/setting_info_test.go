package platformapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestGoAtLeast(t *testing.T) {
	if !goAtLeast(1, 22) {
		t.Fatalf("running %s should satisfy 1.22", runtime.Version())
	}
	if goAtLeast(99, 0) {
		t.Fatal("future major must fail")
	}
}

func TestSystemInfoIsGoNotPHP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/setting.system.system/info", nil)
	SystemInfo(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "PHP版本") || strings.Contains(body, "8.0版本以上") {
		t.Fatalf("still PHP copy: %s", body)
	}
	var wrap struct {
		Data struct {
			Server []map[string]any `json:"server"`
			Env    []map[string]any `json:"env"`
			Auth   []map[string]any `json:"auth"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatal(err)
	}
	params := map[string]string{}
	for _, row := range wrap.Data.Server {
		params[fmtString(row["param"])] = fmtString(row["value"])
	}
	if params["Go版本"] == "" || !strings.HasPrefix(params["Go版本"], "go") {
		t.Fatalf("Go版本 %+v", wrap.Data.Server)
	}
	if params["web服务器环境"] != "Go net/http" {
		t.Fatalf("web %+v", wrap.Data.Server)
	}
	opts := map[string]bool{}
	for _, row := range wrap.Data.Env {
		opts[fmtString(row["option"])] = true
	}
	if !opts["Go版本"] || !opts["MySQL"] {
		t.Fatalf("env %+v", wrap.Data.Env)
	}
	dirs := map[string]bool{}
	for _, row := range wrap.Data.Auth {
		dirs[fmtString(row["dir"])] = true
	}
	if !dirs["/runtime"] || !dirs["config.yaml"] || !dirs["/public/uploads"] {
		t.Fatalf("auth %+v", wrap.Data.Auth)
	}
}

func fmtString(v any) string {
	s, _ := v.(string)
	return s
}
