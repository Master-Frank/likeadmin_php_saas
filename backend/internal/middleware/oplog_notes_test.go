package middleware

import "testing"

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
	if got != "unknown.ctrl/foo" {
		t.Fatalf("%q", got)
	}
}
