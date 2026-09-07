package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestActionNotesPHPNotes(t *testing.T) {
	got := ActionNotes("setting.system.log", "lists")
	if got != " 查看系统日志列表" {
		t.Fatalf("%q", got)
	}
	got = ActionNotes("tenant.tenant_admin", "delete")
	if got != " 删除租户管理员账号" {
		t.Fatalf("%q", got)
	}
	got = ActionNotes("tenant.tenantadmin", "edit")
	if got != " 编辑租户管理员账号" {
		t.Fatalf("%q", got)
	}
}

func TestActionNotesFallback(t *testing.T) {
	got := ActionNotes("unknown.ctrl", "foo")
	if got != "无法获取操作名称，请给控制器方法注释" {
		t.Fatalf("%q", got)
	}
}

func TestRequestAbsoluteURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/auth.admin/detail?id=1", nil)
	c.Request.Host = "127.0.0.1:8080"
	got := requestAbsoluteURL(c)
	if got != "http://127.0.0.1:8080/platformapi/auth.admin/detail?id=1" {
		t.Fatalf("%q", got)
	}
	if requestLogType(c) != "GET" {
		t.Fatal(requestLogType(c))
	}
	c.Request.Method = http.MethodPut
	if requestLogType(c) != "POST" {
		t.Fatal("PHP only records GET/POST")
	}
}

func TestActionNotesAliases(t *testing.T) {
	cases := map[string]string{
		ActionNotes("user.user", "adjustMoney"):                        " 调整用户余额",
		ActionNotes("channel.official_account_menu", "saveAndPublish"): " 保存发布菜单",
		ActionNotes("setting.web.web_setting", "getSiteStatistics"):    " 获取站点统计配置",
		ActionNotes("setting.web.web_setting", "setSiteStatistics"):    " 获取站点统计配置",
		ActionNotes("auth.admin", "all"):                               " 获取管理员数据",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	}
}
