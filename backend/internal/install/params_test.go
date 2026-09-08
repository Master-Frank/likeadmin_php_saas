package install

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestIdentOK(t *testing.T) {
	if !identOK("likeadmin_install_smoke") || !identOK("xx_") || !identOK("la_") {
		t.Fatal("valid idents")
	}
	if identOK("") || identOK("foo.bar") || identOK("la_`x") || identOK("a-b") {
		t.Fatal("invalid idents")
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

func TestCopyDirChmodAccessToken(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "access_token.txt"), []byte("tok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "readme.txt"), []byte("ok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyDir(src, dest); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dest, "access_token.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o777 {
		t.Fatalf("access_token mode %o", st.Mode().Perm())
	}
}
