package router

import "testing"

func TestFrontendRoutesCovered(t *testing.T) {
	platform := []string{
		"login/account", "login/logout", "config/getconfig", "config/dict", "workbench/index",
		"auth.admin/lists", "auth.admin/all", "auth.admin/add", "auth.admin/edit", "auth.admin/delete",
		"auth.admin/detail", "auth.admin/myself", "auth.admin/editself",
		"auth.menu/lists", "auth.menu/all", "auth.menu/add", "auth.menu/edit", "auth.menu/delete", "auth.menu/detail",
		"auth.role/lists", "auth.role/add", "auth.role/edit", "auth.role/delete", "auth.role/detail", "auth.role/all",
		"dept.jobs/lists", "dept.jobs/add", "dept.jobs/edit", "dept.jobs/delete", "dept.jobs/detail", "dept.jobs/all",
		"dept.dept/lists", "dept.dept/add", "dept.dept/edit", "dept.dept/delete", "dept.dept/detail", "dept.dept/all",
		"tenant.tenant/lists", "tenant.tenant/detail", "tenant.tenant/edit", "tenant.tenant/add", "tenant.tenant/delete",
		"tenant.tenantadmin/lists", "tenant.tenantadmin/detail", "tenant.tenantadmin/edit", "tenant.tenantadmin/add", "tenant.tenantadmin/delete",
		"tenant.tenantuser/lists", "tenant.tenantuser/detail",
		"setting.web.web_setting/getcopyright", "setting.web.web_setting/setcopyright",
		"setting.web.web_setting/getwebsite", "setting.web.web_setting/setwebsite",
		"setting.web.web_setting/getagreement", "setting.web.web_setting/setagreement",
		"setting.system.system/info", "setting.system.log/lists", "setting.system.cache/clear",
		"crontab.crontab/lists", "crontab.crontab/add", "crontab.crontab/detail", "crontab.crontab/edit",
		"crontab.crontab/delete", "crontab.crontab/expression", "crontab.crontab/operate",
		"setting.dict.dict_type/lists", "setting.dict.dict_type/all", "setting.dict.dict_type/add",
		"setting.dict.dict_type/edit", "setting.dict.dict_type/delete", "setting.dict.dict_type/detail",
		"setting.dict.dict_data/lists", "setting.dict.dict_data/add", "setting.dict.dict_data/edit",
		"setting.dict.dict_data/delete", "setting.dict.dict_data/detail",
		"setting.user.user/getconfig", "setting.user.user/setconfig",
		"setting.user.user/getregisterconfig", "setting.user.user/setregisterconfig",
		"setting.storage/lists", "setting.storage/change", "setting.storage/setup", "setting.storage/detail",
		"setting.pay.pay_way/getpayway", "setting.pay.pay_way/setpayway",
		"setting.pay.pay_config/lists", "setting.pay.pay_config/setconfig", "setting.pay.pay_config/getconfig",
		"setting.hot_search/getconfig", "setting.hot_search/setconfig",
		"finance.account_log/lists", "recharge.recharge/lists", "finance.account_log/getumchangetype",
		"recharge.recharge/refund", "recharge.recharge/refundagain",
		"finance.refund/record", "finance.refund/log", "finance.refund/stat",
		"decorate.page/detail", "decorate.page/save", "decorate.data/article",
		"decorate.tabbar/detail", "decorate.tabbar/save", "decorate.data/pc",
		"file/addcate", "file/editcate", "file/delcate", "file/listcate", "file/lists",
		"file/delete", "file/move", "file/rename",
		"notice.notice/settinglists", "notice.notice/detail", "notice.notice/set",
		"notice.sms_config/getconfig", "notice.sms_config/detail", "notice.sms_config/setconfig",
		"tools.generator/generatetable", "tools.generator/datatable", "tools.generator/selecttable",
		"tools.generator/detail", "tools.generator/synccolumn", "tools.generator/delete",
		"tools.generator/edit", "tools.generator/preview", "tools.generator/generate", "tools.generator/getmodels",
		"tools.generator/download",
		"article.articlecate/lists", "article.articlecate/all", "article.articlecate/add",
		"article.article/lists", "article/all",
		"download/export",
		"upgrade.upgrade/lists", "upgrade.upgrade/upgrade", "upgrade.upgrade/downloadpkg",
		"channel.official_account_setting/getconfig", "channel.official_account_menu/detail",
		"channel.official_account_reply/lists", "channel.official_account_reply/sort",
		"recharge.recharge/getconfig", "recharge.recharge/setconfig",
	}
	tenant := []string{
		"login/account", "config/getconfig", "workbench/index",
		"auth.admin/lists", "auth.admin/all", "auth.admin/myself",
		"user.user/lists", "user.user/detail", "user.user/edit", "user.user/adjustmoney",
		"article.article/lists", "article.article/add", "article.article/edit", "article.article/delete",
		"article.article/detail", "article.article/updatestatus",
		"article.articlecate/lists", "article.articlecate/add", "article.articlecate/edit",
		"article.articlecate/delete", "article.articlecate/detail", "article.articlecate/updatestatus",
		"article.articlecate/all",
		"decorate.data/article", "decorate.data/pc",
		"setting.web.web_setting/getcopyright", "setting.web.web_setting/getsitestatistics",
		"setting.user.user/getconfig", "setting.storage/lists", "setting.storage/detail",
		"setting.storage/setup", "setting.storage/change",
		"setting.dict.dict_type/add", "setting.dict.dict_type/edit", "setting.dict.dict_type/delete",
		"setting.dict.dict_data/add", "setting.dict.dict_data/edit", "setting.dict.dict_data/delete",
		"setting.pay.pay_way/getpayway",
		"setting.pay.pay_config/lists", "crontab.crontab/lists",
		"notice.notice/settinglists", "finance.account_log/getumchangetype",
		"recharge.recharge/refund", "finance.refund/log",
		"channel.official_account_menu/save", "channel.official_account_reply/add",
		"channel.official_account_reply/sort", "dept.dept/leaderdept",
		"file/lists", "upload/image", "download/export",
	}
	api := []string{
		"index/index", "index/config", "index/policy", "index/decorate",
		"pc/index", "pc/config", "pc/infocenter", "pc/articledetail",
		"login/account", "login/register", "login/logout",
		"login/codeurl", "login/oalogin", "login/mnplogin",
		"login/getscancode", "login/scanlogin", "login/mnpauthbind", "login/oaauthbind", "login/updateuser",
		"user/center", "user/info", "user/setinfo", "user/bindmobile",
		"user/changepassword", "user/resetpassword",
		"article/lists", "article/cate", "article/detail",
		"article/addcollect", "article/cancelcollect", "article/collect",
		"search/hotlists", "recharge/config", "recharge/lists", "recharge/recharge",
		"accountlog/lists", "account_log/lists",
		"sms/sendcode", "wechat/jsconfig", "upload/image",
		"pay/payway", "pay/prepay", "pay/paystatus",
		"pay/notifymnp", "pay/notifyoa", "pay/notifyapp", "pay/alinotify",
	}
	assertCovered(t, "platform", platformRoutes(), platform)
	assertCovered(t, "tenant", tenantRoutes(), tenant)
	assertCovered(t, "api", apiRoutes(), api)
}

func assertCovered(t *testing.T, app string, routes map[string]Handler, want []string) {
	t.Helper()
	for _, key := range want {
		if lookup(routes, key) == nil {
			t.Errorf("%s missing route %s", app, key)
		}
	}
}
