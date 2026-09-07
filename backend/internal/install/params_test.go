package install

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func TestCheckPortClosed(t *testing.T) {
	if err := CheckPort("127.0.0.1", 1); err == nil {
		t.Fatal("closed port should fail")
	}
}

func TestCheckParams(t *testing.T) {
	if CheckParams(map[string]any{}) != "数据表前缀不能为空" {
		t.Fatal(CheckParams(map[string]any{}))
	}
	if CheckParams(map[string]any{"prefix": "la_"}) != "请填写管理员用户名" {
		t.Fatal(CheckParams(map[string]any{"prefix": "la_"}))
	}
	if CheckParams(map[string]any{"prefix": "la_", "admin_user": "admin"}) != "管理员密码不能为空" {
		t.Fatal("empty pass")
	}
	if CheckParams(map[string]any{"prefix": "la_", "admin_user": "admin", "admin_password": "  "}) != "管理员密码不能为空" {
		t.Fatal("blank pass")
	}
	if CheckParams(map[string]any{
		"prefix": "la_", "admin_user": "admin", "admin_password": "a", "admin_confirm_password": "b",
	}) != "两次密码不一致" {
		t.Fatal("mismatch")
	}
	if CheckParams(map[string]any{
		"prefix": "la_", "admin_user": "admin", "admin_password": "a", "admin_confirm_password": "a",
	}) != "" {
		t.Fatal("ok")
	}
}

func TestInstallReadsBodyOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost,
		"/install?prefix=la_&admin_user=hack&admin_password=likeadmin&admin_confirm_password=likeadmin",
		bytes.NewBufferString(`{"prefix":"xx_"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	p := httpx.Body(c)
	if pick(p, "prefix") != "xx_" {
		t.Fatalf("body prefix %v", p)
	}
	if pick(p, "admin_user") != "" {
		t.Fatalf("query admin_user must be ignored, got %v", p)
	}
	if CheckParams(p) != "请填写管理员用户名" {
		t.Fatalf("query-only install fields must not pass CheckParams: %s", CheckParams(p))
	}
}

func TestAccountSalt(t *testing.T) {
	got := AccountSalt(1700000000, "admin")
	want := util.MD5("1700000000admin")[:4]
	if got != want {
		t.Fatalf("salt %s want %s", got, want)
	}
	pwd := util.CreatePassword("likeadmin", got)
	if len(pwd) != 32 || pwd != util.MD5(got+util.MD5("likeadmin"+got)) {
		t.Fatal(pwd)
	}
	if !isOn(map[string]any{"clear_db": "on"}, "clear_db") || isOn(map[string]any{"clear_db": "off"}, "clear_db") {
		t.Fatal("flag")
	}
}
