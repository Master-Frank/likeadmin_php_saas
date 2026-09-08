package router

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode"
)

// TestPHPControllerActionsCovered walks PHP controllers and asserts every
// public action has a Go route. Extra Go routes must match the Vue-shared
// allowlist so we neither omit PHP actions nor silently duplicate them.
func TestPHPControllerActionsCovered(t *testing.T) {
	root := phpAppRoot(t)
	apps := []struct {
		name   string
		routes map[string]Handler
	}{
		{"platformapi", platformRoutes()},
		{"tenantapi", tenantRoutes()},
		{"api", apiRoutes()},
	}
	total := 0
	for _, app := range apps {
		php := scanPHPControllers(t, filepath.Join(root, app.name, "controller"))
		total += len(php)
		phpCompact := map[string]string{}
		for _, act := range php {
			key := strings.ToLower(act.ctrl + "/" + act.action)
			phpCompact[compactKey(key)] = key
			if lookup(app.routes, key) == nil {
				t.Errorf("%s missing Go route for PHP %s (%s)", app.name, key, act.file)
			}
		}
		for key := range app.routes {
			if _, ok := phpCompact[compactKey(key)]; ok {
				continue
			}
			if !allowedExtraGoRoute(app.name, key) {
				t.Errorf("%s extra Go route %s is not in the Vue-shared allowlist", app.name, key)
			}
		}
	}
	if total != 307 {
		t.Errorf("PHP public actions = %d, want 307 (platform+tenant+api)", total)
	}
}

func TestPHPNotNeedLoginCovered(t *testing.T) {
	root := phpAppRoot(t)
	goNeed := notNeedLogin()
	for _, app := range []string{"platformapi", "tenantapi", "api"} {
		php := scanPHPControllers(t, filepath.Join(root, app, "controller"))
		for _, act := range php {
			if !act.notNeed {
				continue
			}
			ctrl := strings.ToLower(act.ctrl)
			action := strings.ToLower(act.action)
			found := false
			for gCtrl, actions := range goNeed[app] {
				if compactKey(gCtrl) != compactKey(ctrl) {
					continue
				}
				for _, a := range actions {
					if a == action {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("%s PHP $notNeedLogin %s/%s missing in Go", app, ctrl, action)
			}
		}
	}
}

func TestPHPModuleInventoryMapped(t *testing.T) {
	root := phpAppRoot(t)
	want := phpModuleInventory()
	got := map[string]bool{}
	for _, rel := range want {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("inventory stale, missing PHP %s", rel)
		}
		got[filepath.ToSlash(rel)] = true
	}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if !inventoryPHPFile(rel) {
			return nil
		}
		if !got[rel] {
			t.Errorf("unmapped PHP module file %s (add it to phpModuleInventory, do not re-implement an already-migrated module)", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type phpAction struct {
	ctrl, action, file string
	notNeed            bool
}

var (
	phpPublicFn    = regexp.MustCompile(`(?m)^\s*public\s+function\s+(\w+)\s*\(`)
	phpNotNeedArr  = regexp.MustCompile(`(?s)\$notNeedLogin\s*=\s*\[(.*?)\];`)
	phpQuotedIdent = regexp.MustCompile(`['"](\w+)['"]`)
)

func scanPHPControllers(t *testing.T, dir string) []phpAction {
	t.Helper()
	var out []phpAction
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if !strings.HasSuffix(d.Name(), "Controller.php") {
			return nil
		}
		if strings.HasPrefix(d.Name(), "Base") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		ctrl, err := phpControllerRoute(dir, path)
		if err != nil {
			return err
		}
		need := phpNotNeedSet(string(raw))
		for _, name := range phpPublicFn.FindAllStringSubmatch(string(raw), -1) {
			action := name[1]
			if strings.HasPrefix(action, "__") {
				continue
			}
			out = append(out, phpAction{
				ctrl:    ctrl,
				action:  action,
				file:    path,
				notNeed: need[strings.ToLower(action)],
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func phpControllerRoute(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}
	rel = strings.TrimSuffix(filepath.ToSlash(rel), "Controller.php")
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = camelToSnake(p)
	}
	return strings.Join(parts, "."), nil
}

func phpNotNeedSet(src string) map[string]bool {
	out := map[string]bool{}
	m := phpNotNeedArr.FindStringSubmatch(src)
	if len(m) < 2 {
		return out
	}
	for _, q := range phpQuotedIdent.FindAllStringSubmatch(m[1], -1) {
		out[strings.ToLower(q[1])] = true
	}
	return out
}

func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func compactKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), "_", "")
}

func allowedExtraGoRoute(app, key string) bool {
	k := compactKey(key)
	switch app {
	case "platformapi":
		// PHP TenantAdminController::add is commented out; Vue still posts it.
		// article/all is tenant-owned but registered on both apps.
		if k == "tenant.tenantadmin/add" || k == "auth.admin/all" || k == "article/all" {
			return true
		}
		// Platform Vue reuses tenant-owned APIs (PHP backends were split).
		for _, p := range []string{
			"setting.hotsearch/", "decorate.", "article.", "channel.",
			"finance.", "recharge.",
		} {
			if strings.HasPrefix(k, compactKey(p)) || strings.HasPrefix(key, p) {
				return true
			}
		}
	case "tenantapi":
		if k == "auth.admin/all" || k == "article/all" {
			return true
		}
		// Tenant Vue reuses platform-owned APIs.
		for _, p := range []string{
			"setting.storage/", "setting.dict.", "crontab.", "tools.generator/",
			"setting.system.log/", "setting.system.system/",
		} {
			if strings.HasPrefix(k, compactKey(p)) || strings.HasPrefix(key, p) {
				return true
			}
		}
	case "api":
		// WeChatPayService notify URL; PHP PayController has no notifyApp.
		return k == "pay/notifyapp" || k == "accountlog/lists"
	}
	return false
}

func phpAppRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for d := wd; ; d = filepath.Dir(d) {
		p := filepath.Join(d, "server", "app")
		if _, err := os.Stat(filepath.Join(p, "platformapi", "controller")); err == nil {
			return p
		}
		if d == filepath.Dir(d) {
			t.Fatal("server/app not found")
		}
	}
}

func inventoryPHPFile(rel string) bool {
	if strings.Contains(rel, "/controller/Base") {
		return false
	}
	switch {
	case strings.Contains(rel, "/logic/") && strings.HasSuffix(rel, "Logic.php"):
		return filepath.Base(rel) != "BaseLogic.php"
	case strings.Contains(rel, "/lists/") && strings.HasSuffix(rel, "Lists.php"):
		base := filepath.Base(rel)
		return !strings.HasPrefix(base, "Base")
	case strings.Contains(rel, "/validate/") && strings.HasSuffix(rel, ".php"):
		return filepath.Base(rel) != "BaseValidate.php"
	case strings.Contains(rel, "/http/middleware/") && strings.HasSuffix(rel, ".php"):
		return true
	case strings.HasPrefix(rel, "common/service/") && strings.HasSuffix(rel, ".php"):
		return true
	case strings.HasPrefix(rel, "common/cache/") && strings.HasSuffix(rel, ".php"):
		return filepath.Base(rel) != "BaseCache.php"
	case strings.HasPrefix(rel, "common/command/") && strings.HasSuffix(rel, ".php"):
		return true
	case strings.HasPrefix(rel, "common/listener/") && strings.HasSuffix(rel, ".php"):
		return true
	case strings.HasSuffix(rel, "/listener/OperationLog.php"):
		return true
	case strings.HasSuffix(rel, "/service/TenantCreatService.php"),
		strings.HasSuffix(rel, "/service/AdminTokenService.php"),
		strings.HasSuffix(rel, "/service/TenantTokenService.php"),
		strings.HasSuffix(rel, "/service/UserTokenService.php"),
		strings.HasSuffix(rel, "/service/WechatUserService.php"):
		return true
	}
	return false
}

// phpModuleInventory is the 1:1 PHP→Go module map. Add a row when PHP grows;
// do not re-add a file that is already listed.
func phpModuleInventory() []string {
	return []string{
		// platformapi logic
		"platformapi/logic/LoginLogic.php",
		"platformapi/logic/ConfigLogic.php",
		"platformapi/logic/WorkbenchLogic.php",
		"platformapi/logic/FileLogic.php",
		"platformapi/logic/auth/AdminLogic.php",
		"platformapi/logic/auth/AuthLogic.php",
		"platformapi/logic/auth/MenuLogic.php",
		"platformapi/logic/auth/RoleLogic.php",
		"platformapi/logic/dept/DeptLogic.php",
		"platformapi/logic/dept/JobsLogic.php",
		"platformapi/logic/crontab/CrontabLogic.php",
		"platformapi/logic/notice/NoticeLogic.php",
		"platformapi/logic/notice/SmsConfigLogic.php",
		"platformapi/logic/setting/CustomerServiceLogic.php",
		"platformapi/logic/setting/StorageLogic.php",
		"platformapi/logic/setting/TransactionSettingsLogic.php",
		"platformapi/logic/setting/dict/DictDataLogic.php",
		"platformapi/logic/setting/dict/DictTypeLogic.php",
		"platformapi/logic/setting/pay/PayConfigLogic.php",
		"platformapi/logic/setting/pay/PayWayLogic.php",
		"platformapi/logic/setting/system/CacheLogic.php",
		"platformapi/logic/setting/system/SystemLogic.php",
		"platformapi/logic/setting/user/UserLogic.php",
		"platformapi/logic/setting/web/WebSettingLogic.php",
		"platformapi/logic/tenant/TenantAdminLogic.php",
		"platformapi/logic/tenant/TenantLogic.php",
		"platformapi/logic/tenant/TenantSystemMenuLogic.php",
		"platformapi/logic/tools/GeneratorLogic.php",
		"platformapi/logic/upgrade/UpgradeLogic.php",
		// tenantapi logic
		"tenantapi/logic/LoginLogic.php",
		"tenantapi/logic/ConfigLogic.php",
		"tenantapi/logic/WorkbenchLogic.php",
		"tenantapi/logic/FileLogic.php",
		"tenantapi/logic/user/UserLogic.php",
		"tenantapi/logic/auth/AdminLogic.php",
		"tenantapi/logic/auth/AuthLogic.php",
		"tenantapi/logic/auth/MenuLogic.php",
		"tenantapi/logic/auth/RoleLogic.php",
		"tenantapi/logic/dept/DeptLogic.php",
		"tenantapi/logic/dept/JobsLogic.php",
		"tenantapi/logic/article/ArticleLogic.php",
		"tenantapi/logic/article/ArticleCateLogic.php",
		"tenantapi/logic/decorate/DecorateDataLogic.php",
		"tenantapi/logic/decorate/DecoratePageLogic.php",
		"tenantapi/logic/decorate/DecorateTabbarLogic.php",
		"tenantapi/logic/channel/AppSettingLogic.php",
		"tenantapi/logic/channel/MnpSettingsLogic.php",
		"tenantapi/logic/channel/OfficialAccountMenuLogic.php",
		"tenantapi/logic/channel/OfficialAccountReplyLogic.php",
		"tenantapi/logic/channel/OfficialAccountSettingLogic.php",
		"tenantapi/logic/channel/OpenSettingLogic.php",
		"tenantapi/logic/channel/WebPageSettingLogic.php",
		"tenantapi/logic/finance/RefundLogic.php",
		"tenantapi/logic/recharge/RechargeLogic.php",
		"tenantapi/logic/notice/NoticeLogic.php",
		"tenantapi/logic/notice/SmsConfigLogic.php",
		"tenantapi/logic/setting/CustomerServiceLogic.php",
		"tenantapi/logic/setting/HotSearchLogic.php",
		"tenantapi/logic/setting/TransactionSettingsLogic.php",
		"tenantapi/logic/setting/pay/PayConfigLogic.php",
		"tenantapi/logic/setting/pay/PayWayLogic.php",
		"tenantapi/logic/setting/system/CacheLogic.php",
		"tenantapi/logic/setting/user/UserLogic.php",
		"tenantapi/logic/setting/web/WebSettingLogic.php",
		// api + common logic
		"api/logic/ArticleLogic.php",
		"api/logic/IndexLogic.php",
		"api/logic/LoginLogic.php",
		"api/logic/PcLogic.php",
		"api/logic/RechargeLogic.php",
		"api/logic/SearchLogic.php",
		"api/logic/SmsLogic.php",
		"api/logic/UserLogic.php",
		"api/logic/WechatLogic.php",
		"common/logic/AccountLogLogic.php",
		"common/logic/NoticeLogic.php",
		"common/logic/PayNotifyLogic.php",
		"common/logic/PaymentLogic.php",
		"common/logic/RefundLogic.php",
		// lists
		"platformapi/lists/auth/AdminLists.php",
		"platformapi/lists/auth/MenuLists.php",
		"platformapi/lists/auth/RoleLists.php",
		"platformapi/lists/crontab/CrontabLists.php",
		"platformapi/lists/dept/JobsLists.php",
		"platformapi/lists/file/FileCateLists.php",
		"platformapi/lists/file/FileLists.php",
		"platformapi/lists/notice/NoticeSettingLists.php",
		"platformapi/lists/setting/dict/DictDataLists.php",
		"platformapi/lists/setting/dict/DictTypeLists.php",
		"platformapi/lists/setting/pay/PayConfigLists.php",
		"platformapi/lists/setting/system/LogLists.php",
		"platformapi/lists/tenant/TenantAdminLists.php",
		"platformapi/lists/tenant/TenantLists.php",
		"platformapi/lists/tools/DataTableLists.php",
		"platformapi/lists/tools/GenerateTableLists.php",
		"platformapi/lists/upgrade/UpgradeLists.php",
		"tenantapi/lists/article/ArticleCateLists.php",
		"tenantapi/lists/article/ArticleLists.php",
		"tenantapi/lists/auth/AdminLists.php",
		"tenantapi/lists/auth/MenuLists.php",
		"tenantapi/lists/auth/RoleLists.php",
		"tenantapi/lists/channel/OfficialAccountReplyLists.php",
		"tenantapi/lists/dept/JobsLists.php",
		"tenantapi/lists/file/FileCateLists.php",
		"tenantapi/lists/file/FileLists.php",
		"tenantapi/lists/finance/AccountLogLists.php",
		"tenantapi/lists/finance/RefundLogLists.php",
		"tenantapi/lists/finance/RefundRecordLists.php",
		"tenantapi/lists/notice/NoticeSettingLists.php",
		"tenantapi/lists/recharge/RechargeLists.php",
		"tenantapi/lists/setting/pay/PayConfigLists.php",
		"tenantapi/lists/user/UserLists.php",
		"api/lists/AccountLogLists.php",
		"api/lists/article/ArticleCollectLists.php",
		"api/lists/article/ArticleLists.php",
		"api/lists/recharge/RechargeLists.php",
		// validate
		"platformapi/validate/LoginValidate.php",
		"platformapi/validate/FileValidate.php",
		"platformapi/validate/auth/AdminValidate.php",
		"platformapi/validate/auth/editSelfValidate.php",
		"platformapi/validate/auth/MenuValidate.php",
		"platformapi/validate/auth/RoleValidate.php",
		"platformapi/validate/crontab/CrontabValidate.php",
		"platformapi/validate/dept/DeptValidate.php",
		"platformapi/validate/dept/JobsValidate.php",
		"platformapi/validate/dict/DictDataValidate.php",
		"platformapi/validate/dict/DictTypeValidate.php",
		"platformapi/validate/notice/NoticeValidate.php",
		"platformapi/validate/notice/SmsConfigValidate.php",
		"platformapi/validate/setting/PayConfigValidate.php",
		"platformapi/validate/setting/StorageValidate.php",
		"platformapi/validate/setting/TransactionSettingsValidate.php",
		"platformapi/validate/setting/UserConfigValidate.php",
		"platformapi/validate/setting/WebSettingValidate.php",
		"platformapi/validate/tenant/TenantAdminValidate.php",
		"platformapi/validate/tenant/TenantValidate.php",
		"platformapi/validate/tools/EditTableValidate.php",
		"platformapi/validate/tools/GenerateTableValidate.php",
		"platformapi/validate/upgrade/UpgradeValidate.php",
		"platformapi/validate/upgrade/downloadPkgValidate.php",
		"tenantapi/validate/LoginValidate.php",
		"tenantapi/validate/FileValidate.php",
		"tenantapi/validate/auth/AdminValidate.php",
		"tenantapi/validate/auth/editSelfValidate.php",
		"tenantapi/validate/auth/MenuValidate.php",
		"tenantapi/validate/auth/RoleValidate.php",
		"tenantapi/validate/article/ArticleCateValidate.php",
		"tenantapi/validate/article/ArticleValidate.php",
		"tenantapi/validate/channel/MnpSettingsValidate.php",
		"tenantapi/validate/channel/OfficialAccountReplyValidate.php",
		"tenantapi/validate/channel/OfficialAccountSettingValidate.php",
		"tenantapi/validate/channel/OpenSettingValidate.php",
		"tenantapi/validate/channel/WebPageSettingValidate.php",
		"tenantapi/validate/decorate/DecoratePageValidate.php",
		"tenantapi/validate/dept/DeptValidate.php",
		"tenantapi/validate/dept/JobsValidate.php",
		"tenantapi/validate/notice/NoticeValidate.php",
		"tenantapi/validate/notice/SmsConfigValidate.php",
		"tenantapi/validate/recharge/RechargeRefundValidate.php",
		"tenantapi/validate/setting/PayConfigValidate.php",
		"tenantapi/validate/setting/StorageValidate.php",
		"tenantapi/validate/setting/TransactionSettingsValidate.php",
		"tenantapi/validate/setting/UserConfigValidate.php",
		"tenantapi/validate/setting/WebSettingValidate.php",
		"tenantapi/validate/user/AdjustUserMoney.php",
		"tenantapi/validate/user/UserValidate.php",
		"api/validate/LoginAccountValidate.php",
		"api/validate/PasswordValidate.php",
		"api/validate/PayValidate.php",
		"api/validate/RechargeValidate.php",
		"api/validate/RegisterValidate.php",
		"api/validate/SendSmsValidate.php",
		"api/validate/SetUserInfoValidate.php",
		"api/validate/UserValidate.php",
		"api/validate/WebScanLoginValidate.php",
		"api/validate/WechatLoginValidate.php",
		"api/validate/WechatValidate.php",
		"common/validate/ListsValidate.php",
		// middleware
		"platformapi/http/middleware/AuthMiddleware.php",
		"platformapi/http/middleware/CheckDemoMiddleware.php",
		"platformapi/http/middleware/EncryDemoDataMiddleware.php",
		"platformapi/http/middleware/InitMiddleware.php",
		"platformapi/http/middleware/LoginMiddleware.php",
		"tenantapi/http/middleware/AuthMiddleware.php",
		"tenantapi/http/middleware/CheckDemoMiddleware.php",
		"tenantapi/http/middleware/EncryDemoDataMiddleware.php",
		"tenantapi/http/middleware/InitMiddleware.php",
		"tenantapi/http/middleware/LoginMiddleware.php",
		"api/http/middleware/InitMiddleware.php",
		"api/http/middleware/LoginMiddleware.php",
		"common/http/middleware/LikeAdminAllowMiddleware.php",
		"common/http/middleware/BaseMiddleware.php",
		// services
		"platformapi/service/AdminTokenService.php",
		"platformapi/service/TenantCreatService.php",
		"tenantapi/service/TenantTokenService.php",
		"api/service/UserTokenService.php",
		"api/service/WechatUserService.php",
		"common/service/ConfigService.php",
		"common/service/FileService.php",
		"common/service/JsonService.php",
		"common/service/UploadService.php",
		"common/service/generator/GenerateService.php",
		"common/service/generator/core/BaseGenerator.php",
		"common/service/generator/core/ControllerGenerator.php",
		"common/service/generator/core/GenerateInterface.php",
		"common/service/generator/core/ListsGenerator.php",
		"common/service/generator/core/LogicGenerator.php",
		"common/service/generator/core/ModelGenerator.php",
		"common/service/generator/core/SqlGenerator.php",
		"common/service/generator/core/ValidateGenerator.php",
		"common/service/generator/core/VueApiGenerator.php",
		"common/service/generator/core/VueEditGenerator.php",
		"common/service/generator/core/VueIndexGenerator.php",
		"common/service/pay/AliPayService.php",
		"common/service/pay/BasePayService.php",
		"common/service/pay/WeChatPayService.php",
		"common/service/sms/SmsDriver.php",
		"common/service/sms/SmsMessageService.php",
		"common/service/sms/engine/AliSms.php",
		"common/service/sms/engine/TencentSms.php",
		"common/service/storage/Driver.php",
		"common/service/storage/engine/Aliyun.php",
		"common/service/storage/engine/Local.php",
		"common/service/storage/engine/Qcloud.php",
		"common/service/storage/engine/Qiniu.php",
		"common/service/storage/engine/Server.php",
		"common/service/wechat/WeChatConfigService.php",
		"common/service/wechat/WeChatMnpService.php",
		"common/service/wechat/WeChatOaService.php",
		"common/service/wechat/WeChatRequestService.php",
		// cache / command / listener
		"common/cache/AdminAccountSafeCache.php",
		"common/cache/AdminAuthCache.php",
		"common/cache/AdminTokenCache.php",
		"common/cache/ExportCache.php",
		"common/cache/TenantAdminAuthCache.php",
		"common/cache/TenantAdminTokenCache.php",
		"common/cache/UserAccountSafeCache.php",
		"common/cache/UserTokenCache.php",
		"common/cache/WebScanLoginCache.php",
		"common/command/Crontab.php",
		"common/command/QueryRefund.php",
		"common/listener/NoticeListener.php",
		"platformapi/listener/OperationLog.php",
		"tenantapi/listener/OperationLog.php",
	}
}
