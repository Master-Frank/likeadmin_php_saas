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
