package platformapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

func TestSmsNameDesc(t *testing.T) {
	if smsNameDesc("ali") != "阿里云短信" || smsNameDesc("ALI") != "阿里云短信" {
		t.Fatal(smsNameDesc("ali"))
	}
	if smsNameDesc("tencent") != "腾讯云短信" {
		t.Fatal(smsNameDesc("tencent"))
	}
	if smsNameDesc("unknown") != "" {
		t.Fatal("unknown type must wipe name like PHP getNameDesc")
	}
}

func TestCrontabWriteCheck(t *testing.T) {
	if crontabWriteCheck(map[string]any{}, false) != "请输入定时任务名称" {
		t.Fatal(crontabWriteCheck(map[string]any{}, false))
	}
	base := map[string]any{"name": "job", "type": nil, "command": "x", "status": 1, "expression": "* * * * *"}
	if crontabWriteCheck(base, false) != "请选择类型" {
		t.Fatal(crontabWriteCheck(base, false))
	}
	base["type"] = 1
	base["status"] = nil
	if crontabWriteCheck(base, false) != "请选择状态" {
		t.Fatal(crontabWriteCheck(base, false))
	}
	base["status"] = 1
	base["expression"] = nil
	if crontabWriteCheck(base, false) != "请输入运行规则" {
		t.Fatal(crontabWriteCheck(base, false))
	}
	base["expression"] = "* * * * *"
	if crontabWriteCheck(base, true) != "参数缺失" {
		t.Fatal(crontabWriteCheck(base, true))
	}
	base["id"] = 1
	if crontabWriteCheck(base, true) != "" {
		t.Fatal(crontabWriteCheck(base, true))
	}
	base["name"] = "   "
	if crontabWriteCheck(base, false) != "" {
		t.Fatal("ThinkPHP require accepts whitespace name")
	}
}

func TestCrontabExpressionInvalidUsesDataEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/crontab.crontab/expression?expression=not-a-cron", nil)
	CrontabExpression(c)
	var body response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != response.CodeOK || body.Msg != "" {
		t.Fatalf("PHP data() envelope, got code=%d msg=%q body=%s", body.Code, body.Msg, w.Body.String())
	}
	s, _ := body.Data.(string)
	if s == "" {
		t.Fatalf("data should be error string, got %#v", body.Data)
	}
}

func TestTenantDetailMissingID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tenant.tenant/detail", nil)
	TenantDetail(c)
	var body response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Msg != "请选择用户" {
		t.Fatalf("missing id must match TenantValidate, got %q body=%s", body.Msg, w.Body.String())
	}
}
