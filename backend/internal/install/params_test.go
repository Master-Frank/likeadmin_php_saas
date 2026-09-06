package install

import (
	"testing"

	"likeadmin/backend/internal/util"
)

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
