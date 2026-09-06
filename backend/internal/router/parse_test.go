package router

import "testing"

func TestParsePath(t *testing.T) {
	c, a := parsePath("/auth.admin/mySelf")
	if c != "auth.admin" || a != "myself" {
		t.Fatalf("got %s %s", c, a)
	}
	c, a = parsePath("login/account")
	if c != "login" || a != "account" {
		t.Fatalf("got %s %s", c, a)
	}
	c, a = parsePath("setting.web.web_setting/getWebsite")
	if c != "setting.web.web_setting" || a != "getwebsite" {
		t.Fatalf("got %s %s", c, a)
	}
}
