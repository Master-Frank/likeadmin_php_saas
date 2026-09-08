package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/cron"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/export"
	"likeadmin/backend/internal/gencrud"
	"likeadmin/backend/internal/install"
	"likeadmin/backend/internal/middleware"
	"likeadmin/backend/internal/openapi"
	"likeadmin/backend/internal/platformapi"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantapi"
	"likeadmin/backend/internal/tenantdb"

	"github.com/gin-gonic/gin"
)

type Handler = gin.HandlerFunc

func New() *gin.Engine {
	if !config.C.App.Debug {
		gin.SetMode(gin.ReleaseMode)
	}
	tenantdb.Register(bootstrap.DB)
	r := gin.New()
	r.Use(gin.Recovery(), middleware.CORS(), middleware.InstallAndTenant())
	response.ExportHook = func(c *gin.Context, rows any, count int64) bool {
		c.Set("likeadmin.export_count", count)
		return export.Maybe(c, "export", rows)
	}

	notNeed := notNeedLogin()

	r.Any("/platformapi/*path", dispatch("platformapi", platformRoutes(), notNeed["platformapi"]))
	r.Any("/tenantapi/*path", dispatch("tenantapi", tenantRoutes(), notNeed["tenantapi"]))
	r.Any("/api/*path", dispatch("api", apiRoutes(), notNeed["api"]))

	r.GET("/crontab", func(c *gin.Context) {
		cron.RunOnce()
		c.String(http.StatusOK, "ok")
	})
	r.GET("/install", install.Wizard)
	r.GET("/install/", install.Wizard)
	// PHP public/index.php and Vue NOT_INSTALL still jump here; nginx /install is already Go.
	r.GET("/install/install.php", install.Wizard)
	r.GET("/install/env", install.Env)
	r.GET("/install/check", install.Status)
	r.POST("/install", install.Run)
	r.Any("/install/status", install.Status)

	spa := func(dir string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.File(filepath.Join(config.C.App.PublicDir, dir, "index.html"))
		}
	}
	if config.C.App.PublicDir != "" {
		r.GET("/", func(c *gin.Context) {
			index := filepath.Join(config.C.App.PublicDir, "index.html")
			if _, err := os.Stat(index); err == nil {
				c.File(index)
				return
			}
			c.Status(http.StatusNotFound)
		})
	}
	r.GET("/platform", spa("platform"))
	r.GET("/platform/*any", spa("platform"))
	r.GET("/admin", spa("admin"))
	r.GET("/admin/*any", spa("admin"))
	r.GET("/mobile", spa("mobile"))
	r.GET("/mobile/*any", spa("mobile"))
	r.GET("/pc", spa("pc"))
	r.GET("/pc/*any", spa("pc"))

	if config.C.App.PublicDir != "" {
		r.Static("/resource", filepath.Join(config.C.App.PublicDir, "resource"))
		r.Static("/uploads", filepath.Join(config.C.App.PublicDir, "uploads"))
	}
	return r
}

func dispatch(app string, routes map[string]Handler, notNeed map[string][]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctrl, action := parsePath(c.Param("path"))
		meta := ctxutil.Get(c)
		meta.App = app
		meta.Controller = ctrl
		meta.Action = action
		switch app {
		case "platformapi":
			meta.Source = ctxutil.SourcePlatform
		case "tenantapi":
			meta.Source = ctxutil.SourceTenant
		default:
			meta.Source = ctxutil.SourceUser
		}
		key := strings.ToLower(ctrl + "/" + action)
		h := lookup(routes, key)
		if h == nil && gencrud.Match(app, ctrl, action) {
			h = gencrud.Handle
		}
		if h == nil {
			response.FailCode(c, "controller not exists:"+ctrl, response.CodeNotFound, 0)
			return
		}
		// run login + auth
		chain := []gin.HandlerFunc{middleware.Login(notNeed), middleware.Auth(), middleware.DemoGuard(), middleware.DemoMask(), middleware.OperationLog(), h}
		c.Set("likeadmin.meta", meta)
		idx := 0
		var next func()
		next = func() {
			if idx >= len(chain) || c.IsAborted() {
				return
			}
			cur := chain[idx]
			idx++
			cur(c)
			if !c.IsAborted() && idx < len(chain) {
				next()
			}
		}
		next()
	}
}

// notNeedLogin mirrors PHP controller `$notNeedLogin` (lowercased). Extra
// keys are Vue-shared or notify URLs PHP referenced but never exposed:
// tenant tools.generator/download, api pay/notifyapp.
func notNeedLogin() map[string]map[string][]string {
	return map[string]map[string][]string{
		"platformapi": {
			"login":           {"account"},
			"config":          {"getconfig", "dict"},
			"download":        {"export"},
			"tools.generator": {"download"},
		},
		"tenantapi": {
			"login":                          {"account"},
			"config":                         {"getconfig", "dict"},
			"download":                       {"export"},
			"tools.generator":                {"download"},
			"channel.official_account_reply": {"index"},
		},
		"api": {
			"index":   {"index", "config", "policy", "decorate"},
			"pc":      {"index", "config", "infocenter", "articledetail"},
			"search":  {"hotlists"},
			"login":   {"register", "account", "logout", "codeurl", "oalogin", "mnplogin", "getscancode", "scanlogin"},
			"sms":     {"sendcode"},
			"user":    {"resetpassword"},
			"pay":     {"notifymnp", "notifyoa", "notifyapp", "alinotify"},
			"wechat":  {"jsconfig"},
			"article": {"lists", "cate", "detail"},
		},
	}
}

func lookup(routes map[string]Handler, key string) Handler {
	if h, ok := routes[key]; ok {
		return h
	}
	compact := strings.ReplaceAll(key, "_", "")
	for k, h := range routes {
		if strings.ReplaceAll(k, "_", "") == compact {
			return h
		}
	}
	return nil
}

func parsePath(p string) (ctrl, action string) {
	p = strings.Trim(p, "/")
	if p == "" {
		return "", ""
	}
	parts := strings.Split(p, "/")
	if len(parts) == 1 {
		return strings.ToLower(parts[0]), "index"
	}
	return strings.ToLower(parts[0]), strings.ToLower(parts[1])
}

func platformRoutes() map[string]Handler {
	return map[string]Handler{
		"login/account": platformapi.LoginAccount, "login/logout": platformapi.LoginLogout,
		"config/getconfig": platformapi.ConfigGet, "config/dict": platformapi.ConfigDict,
		"workbench/index":  platformapi.WorkbenchIndex,
		"auth.admin/lists": platformapi.AdminLists, "auth.admin/add": platformapi.AdminAdd,
		"auth.admin/edit": platformapi.AdminEdit, "auth.admin/delete": platformapi.AdminDelete,
		"auth.admin/detail": platformapi.AdminDetail, "auth.admin/myself": platformapi.AdminMySelf,
		"auth.admin/editself": platformapi.AdminEditSelf, "auth.admin/all": platformapi.AdminAll,
		"auth.menu/route": platformapi.MenuRoute, "auth.menu/lists": platformapi.MenuLists,
		"auth.menu/detail": platformapi.MenuDetail, "auth.menu/add": platformapi.MenuAdd,
		"auth.menu/edit": platformapi.MenuEdit, "auth.menu/delete": platformapi.MenuDelete,
		"auth.menu/updatestatus": platformapi.MenuUpdateStatus, "auth.menu/all": platformapi.MenuAll,
		"auth.role/lists": platformapi.RoleLists, "auth.role/add": platformapi.RoleAdd,
		"auth.role/edit": platformapi.RoleEdit, "auth.role/delete": platformapi.RoleDelete,
		"auth.role/detail": platformapi.RoleDetail, "auth.role/all": platformapi.RoleAll,
		"dept.dept/lists": platformapi.DeptLists, "dept.dept/leaderdept": platformapi.DeptLeader,
		"dept.dept/add": platformapi.DeptAdd, "dept.dept/edit": platformapi.DeptEdit,
		"dept.dept/delete": platformapi.DeptDelete, "dept.dept/detail": platformapi.DeptDetail, "dept.dept/all": platformapi.DeptAll,
		"dept.jobs/lists": platformapi.JobsLists, "dept.jobs/add": platformapi.JobsAdd,
		"dept.jobs/edit": platformapi.JobsEdit, "dept.jobs/delete": platformapi.JobsDelete,
		"dept.jobs/detail": platformapi.JobsDetail, "dept.jobs/all": platformapi.JobsAll,
		"file/lists": platformapi.FileLists, "file/move": platformapi.FileMove, "file/rename": platformapi.FileRename,
		"file/delete": platformapi.FileDelete, "file/listcate": platformapi.FileListCate,
		"file/addcate": platformapi.FileAddCate, "file/editcate": platformapi.FileEditCate, "file/delcate": platformapi.FileDelCate,
		"upload/image": platformapi.UploadImage, "upload/video": platformapi.UploadVideo, "upload/file": platformapi.UploadFile,
		"tenant.tenant/lists": platformapi.TenantLists, "tenant.tenant/detail": platformapi.TenantDetail,
		"tenant.tenant/add": platformapi.TenantAdd, "tenant.tenant/edit": platformapi.TenantEdit, "tenant.tenant/delete": platformapi.TenantDelete,
		"tenant.tenantadmin/lists": platformapi.TenantAdminLists, "tenant.tenantadmin/detail": platformapi.TenantAdminDetail,
		"tenant.tenantadmin/add":  platformapi.TenantAdminAdd,
		"tenant.tenantadmin/edit": platformapi.TenantAdminEdit, "tenant.tenantadmin/delete": platformapi.TenantAdminDelete,
		"tenant.tenantuser/lists": platformapi.TenantUserLists, "tenant.tenantuser/detail": platformapi.TenantUserDetail,
		"setting.web.web_setting/getwebsite": platformapi.WebGetWebsite, "setting.web.web_setting/setwebsite": platformapi.WebSetWebsite,
		"setting.web.web_setting/getcopyright": platformapi.WebGetCopyright, "setting.web.web_setting/setcopyright": platformapi.WebSetCopyright,
		"setting.web.web_setting/getagreement": platformapi.WebGetAgreement, "setting.web.web_setting/setagreement": platformapi.WebSetAgreement,
		"setting.user.user/getconfig": platformapi.UserGetConfig, "setting.user.user/setconfig": platformapi.UserSetConfig,
		"setting.user.user/getregisterconfig": platformapi.UserGetRegisterConfig, "setting.user.user/setregisterconfig": platformapi.UserSetRegisterConfig,
		"setting.transaction_settings/getconfig": platformapi.TransactionGet, "setting.transaction_settings/setconfig": platformapi.TransactionSet,
		"setting.customer_service/getconfig": platformapi.CustomerGet, "setting.customer_service/setconfig": platformapi.CustomerSet,
		"setting.system.cache/clear": platformapi.CacheClear, "setting.system.system/info": platformapi.SystemInfo,
		"setting.system.log/lists":     platformapi.LogLists,
		"setting.dict.dict_type/lists": platformapi.DictTypeLists, "setting.dict.dict_type/add": platformapi.DictTypeAdd,
		"setting.dict.dict_type/edit": platformapi.DictTypeEdit, "setting.dict.dict_type/delete": platformapi.DictTypeDelete,
		"setting.dict.dict_type/detail": platformapi.DictTypeDetail, "setting.dict.dict_type/all": platformapi.DictTypeAll,
		"setting.dict.dict_data/lists": platformapi.DictDataLists, "setting.dict.dict_data/add": platformapi.DictDataAdd,
		"setting.dict.dict_data/edit": platformapi.DictDataEdit, "setting.dict.dict_data/delete": platformapi.DictDataDelete,
		"setting.dict.dict_data/detail": platformapi.DictDataDetail,
		"setting.storage/lists":         platformapi.StorageLists, "setting.storage/detail": platformapi.StorageDetail,
		"setting.storage/setup": platformapi.StorageSetup, "setting.storage/change": platformapi.StorageChange,
		"setting.pay.pay_config/lists": platformapi.PayConfigLists, "setting.pay.pay_config/getconfig": platformapi.PayConfigGet,
		"setting.pay.pay_config/setconfig": platformapi.PayConfigSet,
		"setting.pay.pay_way/getpayway":    platformapi.PayWayGet, "setting.pay.pay_way/setpayway": platformapi.PayWaySet,
		"crontab.crontab/lists": platformapi.CrontabLists, "crontab.crontab/add": platformapi.CrontabAdd,
		"crontab.crontab/edit": platformapi.CrontabEdit, "crontab.crontab/delete": platformapi.CrontabDelete,
		"crontab.crontab/operate": platformapi.CrontabOperate, "crontab.crontab/detail": platformapi.CrontabDetail,
		"crontab.crontab/expression": platformapi.CrontabExpression,
		"notice.notice/settinglists": platformapi.NoticeSettingLists, "notice.notice/detail": platformapi.NoticeDetail,
		"notice.notice/set":           platformapi.NoticeSet,
		"notice.sms_config/getconfig": platformapi.SmsConfigGet, "notice.sms_config/setconfig": platformapi.SmsConfigSet,
		"notice.sms_config/detail":  platformapi.SmsConfigDetail,
		"tools.generator/datatable": platformapi.GeneratorDataTable, "tools.generator/generatetable": platformapi.GeneratorGenerateTable,
		"tools.generator/selecttable": platformapi.GeneratorSelectTable, "tools.generator/detail": platformapi.GeneratorDetail,
		"tools.generator/synccolumn": platformapi.GeneratorSyncColumn, "tools.generator/delete": platformapi.GeneratorDelete,
		"tools.generator/edit": platformapi.GeneratorEdit, "tools.generator/preview": platformapi.GeneratorPreview,
		"tools.generator/generate": platformapi.GeneratorGenerate, "tools.generator/getmodels": platformapi.GeneratorGetModels,
		"tools.generator/download": platformapi.GeneratorDownload,
		"download/export":          platformapi.DownloadExport,
		"upgrade.upgrade/lists":    platformapi.UpgradeLists, "upgrade.upgrade/upgrade": platformapi.UpgradeDo,
		"upgrade.upgrade/downloadpkg":  platformapi.UpgradeDownloadPkg,
		"setting.hot_search/getconfig": tenantapi.HotSearchGet, "setting.hot_search/setconfig": tenantapi.HotSearchSet,
		"decorate.page/detail": tenantapi.DecoratePageDetail, "decorate.page/save": tenantapi.DecoratePageSave,
		"decorate.tabbar/detail": tenantapi.DecorateTabbarDetail, "decorate.tabbar/save": tenantapi.DecorateTabbarSave,
		"decorate.data/article": tenantapi.DecorateDataArticle, "decorate.data/pc": tenantapi.DecorateDataPC,
		"article.article/lists": tenantapi.ArticleLists, "article.article/add": tenantapi.ArticleAdd,
		"article.article/edit": tenantapi.ArticleEdit, "article.article/delete": tenantapi.ArticleDelete,
		"article.article/detail": tenantapi.ArticleDetail, "article.article/updatestatus": tenantapi.ArticleUpdateStatus,
		"article.article_cate/lists": tenantapi.ArticleCateLists, "article.article_cate/add": tenantapi.ArticleCateAdd,
		"article.article_cate/edit": tenantapi.ArticleCateEdit, "article.article_cate/delete": tenantapi.ArticleCateDelete,
		"article.article_cate/all": tenantapi.ArticleCateAll, "article.article_cate/detail": tenantapi.ArticleCateDetail,
		"article.article_cate/updatestatus": tenantapi.ArticleCateUpdateStatus, "article/all": tenantapi.ArticleAll,
		"channel.official_account_setting/getconfig": tenantapi.ChannelGetOnly("official_account"),
		"channel.official_account_setting/setconfig": tenantapi.ChannelSetOnly("official_account"),
		"channel.mnp_settings/getconfig":             tenantapi.ChannelGetOnly("mnp"), "channel.mnp_settings/setconfig": tenantapi.ChannelSetOnly("mnp"),
		"channel.open_setting/getconfig": tenantapi.ChannelGetOnly("open"), "channel.open_setting/setconfig": tenantapi.ChannelSetOnly("open"),
		"channel.web_page_setting/getconfig": tenantapi.ChannelGetOnly("h5"), "channel.web_page_setting/setconfig": tenantapi.ChannelSetOnly("h5"),
		"channel.app_setting/getconfig": tenantapi.ChannelGetOnly("app"), "channel.app_setting/setconfig": tenantapi.ChannelSetOnly("app"),
		"channel.official_account_menu/detail": tenantapi.OAMenuDetail, "channel.official_account_menu/save": tenantapi.OAMenuSave,
		"channel.official_account_menu/saveandpublish": tenantapi.OAMenuSaveAndPublish,
		"channel.official_account_reply/lists":         tenantapi.OAReplyLists, "channel.official_account_reply/add": tenantapi.OAReplyAdd,
		"channel.official_account_reply/edit": tenantapi.OAReplyEdit, "channel.official_account_reply/delete": tenantapi.OAReplyDelete,
		"channel.official_account_reply/detail": tenantapi.OAReplyDetail, "channel.official_account_reply/status": tenantapi.OAReplyStatus,
		"channel.official_account_reply/sort": tenantapi.OAReplySort,
		"finance.account_log/lists":           tenantapi.FinanceAccountLogLists, "finance.account_log/getumchangetype": tenantapi.GetUmChangeType,
		"recharge.recharge/lists": tenantapi.RechargeLists, "recharge.recharge/getconfig": tenantapi.RechargeGetConfig,
		"recharge.recharge/setconfig": tenantapi.RechargeSetConfig, "recharge.recharge/refund": tenantapi.RechargeRefund,
		"recharge.recharge/refundagain": tenantapi.RechargeRefundAgain,
		"finance.refund/record":         tenantapi.FinanceRefundRecord, "finance.refund/stat": tenantapi.FinanceRefundStat,
		"finance.refund/log": tenantapi.FinanceRefundLog,
	}
}

func tenantRoutes() map[string]Handler {
	oaGet, oaSet := tenantapi.ChannelGetSet("official_account")
	mnpGet, mnpSet := tenantapi.ChannelGetSet("mnp")
	openGet, openSet := tenantapi.ChannelGetSet("open")
	h5Get, h5Set := tenantapi.ChannelGetSet("h5")
	appGet, appSet := tenantapi.ChannelGetSet("app")
	return map[string]Handler{
		"login/account": tenantapi.LoginAccount, "login/logout": tenantapi.LoginLogout,
		"config/getconfig": tenantapi.ConfigGet, "config/dict": tenantapi.ConfigDict,
		"workbench/index":  tenantapi.WorkbenchIndex,
		"auth.admin/lists": tenantapi.AdminLists, "auth.admin/add": tenantapi.AdminAdd,
		"auth.admin/edit": tenantapi.AdminEdit, "auth.admin/delete": tenantapi.AdminDelete,
		"auth.admin/detail": tenantapi.AdminDetail, "auth.admin/myself": tenantapi.AdminMySelf,
		"auth.admin/editself": tenantapi.AdminEditSelf, "auth.admin/all": tenantapi.AdminAll,
		"auth.menu/route": tenantapi.MenuRoute, "auth.menu/lists": tenantapi.MenuLists,
		"auth.menu/all": tenantapi.MenuAll, "auth.menu/add": tenantapi.MenuAdd,
		"auth.menu/edit": tenantapi.MenuEdit, "auth.menu/delete": tenantapi.MenuDelete,
		"auth.menu/detail": tenantapi.MenuDetail, "auth.menu/updatestatus": tenantapi.MenuUpdateStatus,
		"auth.role/lists": tenantapi.RoleLists, "auth.role/add": tenantapi.RoleAdd,
		"auth.role/edit": tenantapi.RoleEdit, "auth.role/delete": tenantapi.RoleDelete,
		"auth.role/detail": tenantapi.RoleDetail, "auth.role/all": tenantapi.RoleAll,
		"dept.dept/lists": tenantapi.DeptLists, "dept.dept/leaderdept": tenantapi.DeptLeader, "dept.dept/add": tenantapi.DeptAdd,
		"dept.dept/edit": tenantapi.DeptEdit, "dept.dept/delete": tenantapi.DeptDelete,
		"dept.dept/detail": tenantapi.DeptDetail, "dept.dept/all": tenantapi.DeptAll,
		"dept.jobs/lists": tenantapi.JobsLists, "dept.jobs/add": tenantapi.JobsAdd,
		"dept.jobs/edit": tenantapi.JobsEdit, "dept.jobs/delete": tenantapi.JobsDelete,
		"dept.jobs/detail": tenantapi.JobsDetail, "dept.jobs/all": tenantapi.JobsAll,
		"user.user/lists": tenantapi.UserLists, "user.user/detail": tenantapi.UserDetail, "user.user/edit": tenantapi.UserEdit,
		"user.user/adjustmoney": tenantapi.UserAdjustMoney,
		"article.article/lists": tenantapi.ArticleLists, "article.article/add": tenantapi.ArticleAdd,
		"article.article/edit": tenantapi.ArticleEdit, "article.article/delete": tenantapi.ArticleDelete,
		"article.article/detail": tenantapi.ArticleDetail, "article.article/updatestatus": tenantapi.ArticleUpdateStatus,
		"article.article_cate/lists": tenantapi.ArticleCateLists, "article.article_cate/add": tenantapi.ArticleCateAdd,
		"article.article_cate/edit": tenantapi.ArticleCateEdit, "article.article_cate/delete": tenantapi.ArticleCateDelete,
		"article.article_cate/all": tenantapi.ArticleCateAll, "article.article_cate/detail": tenantapi.ArticleCateDetail,
		"article.article_cate/updatestatus": tenantapi.ArticleCateUpdateStatus, "article/all": tenantapi.ArticleAll,
		"decorate.page/detail": tenantapi.DecoratePageDetail, "decorate.page/save": tenantapi.DecoratePageSave,
		"decorate.tabbar/detail": tenantapi.DecorateTabbarDetail, "decorate.tabbar/save": tenantapi.DecorateTabbarSave,
		"decorate.data/article": tenantapi.DecorateDataArticle, "decorate.data/pc": tenantapi.DecorateDataPC,
		"setting.web.web_setting/getwebsite": tenantapi.SettingGetWebsite, "setting.web.web_setting/setwebsite": tenantapi.SettingSetWebsite,
		"setting.web.web_setting/getcopyright": tenantapi.SettingGetCopyright, "setting.web.web_setting/setcopyright": tenantapi.SettingSetCopyright,
		"setting.web.web_setting/getagreement": tenantapi.SettingGetAgreement, "setting.web.web_setting/setagreement": tenantapi.SettingSetAgreement,
		"setting.web.web_setting/getsitestatistics": tenantapi.SettingGetSiteStatistics,
		"setting.web.web_setting/setsitestatistics": tenantapi.SettingSetSiteStatistics,
		"setting.user.user/getconfig":               tenantapi.SettingUserGetConfig, "setting.user.user/setconfig": tenantapi.SettingUserSetConfig,
		"setting.user.user/getregisterconfig": tenantapi.SettingUserGetRegister, "setting.user.user/setregisterconfig": tenantapi.SettingUserSetRegister,
		"setting.transaction_settings/getconfig": tenantapi.SettingTransactionGet, "setting.transaction_settings/setconfig": tenantapi.SettingTransactionSet,
		"setting.customer_service/getconfig": tenantapi.SettingCustomerGet, "setting.customer_service/setconfig": tenantapi.SettingCustomerSet,
		"setting.system.cache/clear": platformapi.CacheClear, "setting.system.system/info": platformapi.SystemInfo,
		"setting.system.log/lists":     platformapi.LogLists,
		"setting.dict.dict_type/lists": platformapi.DictTypeLists, "setting.dict.dict_type/detail": platformapi.DictTypeDetail,
		"setting.dict.dict_type/all": platformapi.DictTypeAll,
		"setting.dict.dict_type/add": platformapi.DictTypeAdd, "setting.dict.dict_type/edit": platformapi.DictTypeEdit,
		"setting.dict.dict_type/delete": platformapi.DictTypeDelete,
		"setting.dict.dict_data/lists":  platformapi.DictDataLists, "setting.dict.dict_data/detail": platformapi.DictDataDetail,
		"setting.dict.dict_data/add": platformapi.DictDataAdd, "setting.dict.dict_data/edit": platformapi.DictDataEdit,
		"setting.dict.dict_data/delete": platformapi.DictDataDelete,
		"setting.storage/lists":         platformapi.StorageLists, "setting.storage/detail": platformapi.StorageDetail,
		"setting.storage/setup": platformapi.StorageSetup, "setting.storage/change": platformapi.StorageChange,
		"setting.pay.pay_config/lists": tenantapi.PayConfigLists, "setting.pay.pay_config/getconfig": tenantapi.PayConfigGet,
		"setting.pay.pay_config/setconfig": tenantapi.PayConfigSet,
		"setting.pay.pay_way/getpayway":    tenantapi.PayWayGet, "setting.pay.pay_way/setpayway": tenantapi.PayWaySet,
		"crontab.crontab/lists": platformapi.CrontabLists, "crontab.crontab/add": platformapi.CrontabAdd,
		"crontab.crontab/edit": platformapi.CrontabEdit, "crontab.crontab/delete": platformapi.CrontabDelete,
		"crontab.crontab/operate": platformapi.CrontabOperate, "crontab.crontab/detail": platformapi.CrontabDetail,
		"crontab.crontab/expression": platformapi.CrontabExpression,
		"tools.generator/datatable":  platformapi.GeneratorDataTable, "tools.generator/generatetable": platformapi.GeneratorGenerateTable,
		"tools.generator/selecttable": platformapi.GeneratorSelectTable, "tools.generator/detail": platformapi.GeneratorDetail,
		"tools.generator/synccolumn": platformapi.GeneratorSyncColumn, "tools.generator/delete": platformapi.GeneratorDelete,
		"tools.generator/edit": platformapi.GeneratorEdit, "tools.generator/preview": platformapi.GeneratorPreview,
		"tools.generator/generate": platformapi.GeneratorGenerate, "tools.generator/getmodels": platformapi.GeneratorGetModels,
		"tools.generator/download":   platformapi.GeneratorDownload,
		"notice.notice/settinglists": tenantapi.TenantNoticeLists, "notice.notice/detail": tenantapi.TenantNoticeDetail,
		"notice.notice/set":           tenantapi.TenantNoticeSet,
		"notice.sms_config/getconfig": platformapi.SmsConfigGet, "notice.sms_config/setconfig": platformapi.SmsConfigSet,
		"notice.sms_config/detail":                   platformapi.SmsConfigDetail,
		"channel.official_account_setting/getconfig": oaGet, "channel.official_account_setting/setconfig": oaSet,
		"channel.mnp_settings/getconfig": mnpGet, "channel.mnp_settings/setconfig": mnpSet,
		"channel.open_setting/getconfig": openGet, "channel.open_setting/setconfig": openSet,
		"channel.web_page_setting/getconfig": h5Get, "channel.web_page_setting/setconfig": h5Set,
		"channel.app_setting/getconfig": appGet, "channel.app_setting/setconfig": appSet,
		"channel.official_account_reply/index": tenantapi.OAReplyIndex,
		"channel.official_account_reply/lists": tenantapi.OAReplyLists, "channel.official_account_reply/add": tenantapi.OAReplyAdd,
		"channel.official_account_reply/edit": tenantapi.OAReplyEdit, "channel.official_account_reply/delete": tenantapi.OAReplyDelete,
		"channel.official_account_reply/detail": tenantapi.OAReplyDetail, "channel.official_account_reply/status": tenantapi.OAReplyStatus,
		"channel.official_account_reply/sort":  tenantapi.OAReplySort,
		"channel.official_account_menu/detail": tenantapi.OAMenuDetail, "channel.official_account_menu/save": tenantapi.OAMenuSave,
		"channel.official_account_menu/saveandpublish": tenantapi.OAMenuSaveAndPublish,
		"setting.hot_search/getconfig":                 tenantapi.HotSearchGet, "setting.hot_search/setconfig": tenantapi.HotSearchSet,
		"recharge.recharge/getconfig": tenantapi.RechargeGetConfig, "recharge.recharge/setconfig": tenantapi.RechargeSetConfig,
		"recharge.recharge/lists": tenantapi.RechargeLists, "recharge.recharge/refund": tenantapi.RechargeRefund,
		"recharge.recharge/refundagain": tenantapi.RechargeRefundAgain,
		"finance.account_log/lists":     tenantapi.FinanceAccountLogLists, "finance.account_log/getumchangetype": tenantapi.GetUmChangeType,
		"finance.refund/record": tenantapi.FinanceRefundRecord, "finance.refund/stat": tenantapi.FinanceRefundStat,
		"finance.refund/log": tenantapi.FinanceRefundLog,
		"upload/image":       tenantapi.UploadImage, "upload/video": tenantapi.UploadVideo, "upload/file": tenantapi.UploadFile,
		"file/lists": tenantapi.FileLists, "file/move": tenantapi.FileMove, "file/rename": tenantapi.FileRename,
		"file/delete": tenantapi.FileDelete, "file/listcate": tenantapi.FileListCate,
		"file/addcate": tenantapi.FileAddCate, "file/editcate": tenantapi.FileEditCate, "file/delcate": tenantapi.FileDelCate,
		"download/export": platformapi.DownloadExport,
	}
}

func apiRoutes() map[string]Handler {
	return map[string]Handler{
		"index/index": openapi.IndexIndex, "index/config": openapi.IndexConfig,
		"index/policy": openapi.IndexPolicy, "index/decorate": openapi.IndexDecorate,
		"pc/index": openapi.PcIndex, "pc/config": openapi.PcConfig,
		"pc/infocenter": openapi.PcInfoCenter, "pc/articledetail": openapi.PcArticleDetail,
		"login/register": openapi.LoginRegister, "login/account": openapi.LoginAccount, "login/logout": openapi.LoginLogout,
		"login/codeurl": openapi.LoginCodeURL, "login/oalogin": openapi.LoginOALogin, "login/mnplogin": openapi.LoginMnpLogin,
		"login/getscancode": openapi.LoginGetScanCode, "login/scanlogin": openapi.LoginScanLogin,
		"login/mnpauthbind": openapi.LoginMnpAuthBind, "login/oaauthbind": openapi.LoginOAAuthBind, "login/updateuser": openapi.LoginUpdateUser,
		"user/center": openapi.UserCenter, "user/info": openapi.UserInfo, "user/setinfo": openapi.UserSetInfo,
		"user/bindmobile": openapi.UserBindMobile, "user/getmobilebymnp": openapi.UserGetMobileByMnp,
		"user/changepassword": openapi.UserChangePassword, "user/resetpassword": openapi.UserResetPassword,
		"article/lists": openapi.ArticleLists, "article/cate": openapi.ArticleCate, "article/detail": openapi.ArticleDetail,
		"article/addcollect": openapi.ArticleAddCollect, "article/cancelcollect": openapi.ArticleCancelCollect,
		"article/collect": openapi.ArticleCollect,
		"search/hotlists": openapi.SearchHot,
		"recharge/config": openapi.RechargeConfig, "recharge/lists": openapi.RechargeLists,
		"recharge/recharge": openapi.RechargeCreate,
		"accountlog/lists":  openapi.AccountLogLists, "account_log/lists": openapi.AccountLogLists,
		"sms/sendcode":    openapi.SmsSendCode,
		"wechat/jsconfig": openapi.WechatJsConfig,
		"upload/image":    openapi.UploadImage,
		"pay/payway":      openapi.PayWay, "pay/prepay": openapi.PayPrepay, "pay/paystatus": openapi.PayStatus,
		"pay/notifymnp": openapi.PayNotifyOK, "pay/notifyoa": openapi.PayNotifyOK, "pay/notifyapp": openapi.PayNotifyOK, "pay/alinotify": openapi.AliNotify,
	}
}
