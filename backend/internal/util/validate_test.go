package util

import "testing"

func TestValidRegisterAccount(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "请输入账号"},
		{"ab", "账号须为3-12位之间"},
		{"abcdefghijklm", "账号须为3-12位之间"},
		{"abcdef", "账号须为字母数字组合"},
		{"123456", "账号须为字母数字组合"},
		{"ab-12", "账号须为字母数字组合"},
		{"u19426", ""},
		{"pairu1", ""},
	}
	for _, c := range cases {
		if got := ValidRegisterAccount(c.in); got != c.want {
			t.Fatalf("account %q got %q want %q", c.in, got, c.want)
		}
	}
}

func TestValidRegisterPassword(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "请输入密码"},
		{"ab12", "密码须在6-25位之间"},
		{"likeadmin", "密码须为数字,字母或符号组合"},
		{"12345678", "密码须为数字,字母或符号组合"},
		{"LIKEADMIN", "密码须为数字,字母或符号组合"},
		{"!!!!!!", "密码须为数字,字母或符号组合"},
		{"likeadmin1", ""},
		{"Likeadmin1", ""},
		{"ab!@#$", ""},
		{"123!@#", ""},
		{"Pass word", ""},
		{"密码密码1", "密码须在6-25位之间"},
		{"密码密码a1", ""},
	}
	for _, c := range cases {
		if got := ValidRegisterPassword(c.in); got != c.want {
			t.Fatalf("password %q got %q want %q", c.in, got, c.want)
		}
	}
}

func TestAdminWriteCheck(t *testing.T) {
	if AdminWriteCheck("", "n", "123456", true) != "账号不能为空" {
		t.Fatal(AdminWriteCheck("", "n", "123456", true))
	}
	if AdminWriteCheck("acc", "", "123456", true) != "名称不能为空" {
		t.Fatal(AdminWriteCheck("acc", "", "123456", true))
	}
	if AdminWriteCheck("acc", "name", "123", true) != "密码长度须在6-32位字符" {
		t.Fatal(AdminWriteCheck("acc", "name", "123", true))
	}
	if AdminWriteCheck("acc", "name", "123456", true) != "" {
		t.Fatal("expected ok")
	}
}

func TestValidChinaMobile(t *testing.T) {
	if ValidChinaMobile("") != "请输入内容" {
		t.Fatal(ValidChinaMobile(""))
	}
	if ValidChinaMobile("123") != "手机号码格式错误" {
		t.Fatal(ValidChinaMobile("123"))
	}
	if ValidChinaMobile("13800138000") != "" {
		t.Fatal(ValidChinaMobile("13800138000"))
	}
}

func TestLoginWayAllows(t *testing.T) {
	if !LoginWayAllows([]any{"1", "2"}, 1) {
		t.Fatal("scene 1 should be allowed")
	}
	if LoginWayAllows([]any{"1", "2"}, 3) {
		t.Fatal("scene 3 should be rejected")
	}
	if LoginWayAllows([]any{"1", "2"}, 0) {
		t.Fatal("scene 0 should be rejected")
	}
}

func TestFileNameCheck(t *testing.T) {
	if FileNameCheck("") != "请填写分组名称" {
		t.Fatal(FileNameCheck(""))
	}
	long := "一二三四五六七八九十一二三四五六七八九十X"
	if FileNameCheck(long) != "分组名称长度须为20字符内" {
		t.Fatal(FileNameCheck(long))
	}
	if FileNameCheck("图片") != "" {
		t.Fatal("expected ok")
	}
}

func TestFileMoveCheck(t *testing.T) {
	if FileMoveCheck(map[string]any{}, nil) != "缺少cid参数" {
		t.Fatal("cid first")
	}
	if FileMoveCheck(map[string]any{"ids": []any{1}}, []uint{1}) != "缺少cid参数" {
		t.Fatal("cid")
	}
	if FileMoveCheck(map[string]any{"ids": []any{1}, "cid": 0}, []uint{1}) != "" {
		t.Fatal("cid 0 should be allowed")
	}
	if FileMoveCheck(map[string]any{"cid": "abc", "ids": []any{1}}, []uint{1}) != "cid必须是数字" {
		t.Fatal(FileMoveCheck(map[string]any{"cid": "abc", "ids": []any{1}}, []uint{1}))
	}
	if FileMoveCheck(map[string]any{"cid": 0, "ids": "1"}, nil) != "ids必须是数组" {
		t.Fatal(FileMoveCheck(map[string]any{"cid": 0, "ids": "1"}, nil))
	}
}

func TestFileIDCheck(t *testing.T) {
	if FileIDCheck(map[string]any{}) != "缺少id参数" {
		t.Fatal(FileIDCheck(map[string]any{}))
	}
	if FileIDCheck(map[string]any{"id": "abc"}) != "id必须是数字" {
		t.Fatal(FileIDCheck(map[string]any{"id": "abc"}))
	}
	if FileIDCheck(map[string]any{"id": 0}) != "" {
		t.Fatal("id 0 is a number")
	}
	if FileIDCheck(map[string]any{"id": -1}) != "id必须是数字" {
		t.Fatal(FileIDCheck(map[string]any{"id": -1}))
	}
	if FileMoveCheck(map[string]any{"cid": -1, "ids": []any{1}}, []uint{1}) != "cid必须是数字" {
		t.Fatal(FileMoveCheck(map[string]any{"cid": -1, "ids": []any{1}}, []uint{1}))
	}
}

func TestUserRegisterConfigCheckRequireIfScene(t *testing.T) {
	if msg := UserRegisterConfigCheck(map[string]any{}); msg != "" {
		t.Fatalf("official body without scene %q", msg)
	}
	if msg := UserRegisterConfigCheck(map[string]any{"scene": "register"}); msg != "请选择登录方式" {
		t.Fatalf("scene=register %q", msg)
	}
	if msg := UserRegisterConfigCheck(map[string]any{"scene": "register", "login_way": []any{"1"}}); msg != "请选择注册强制绑定手机" {
		t.Fatalf("missing coerce %q", msg)
	}
	if msg := UserRegisterConfigCheck(map[string]any{
		"scene": "register", "login_way": []any{"1"}, "coerce_mobile": 0,
	}); msg != "" {
		t.Fatalf("ok %q", msg)
	}
}

func TestOAReplyWriteCheck(t *testing.T) {
	if OAReplyWriteCheck(map[string]any{}, false) != "请输入回复类型" {
		t.Fatal(OAReplyWriteCheck(map[string]any{}, false))
	}
	p := map[string]any{"reply_type": 2, "name": "r", "content_type": 1, "content": "hi", "status": 0}
	if OAReplyWriteCheck(p, false) != "请输入关键词" {
		t.Fatal(OAReplyWriteCheck(p, false))
	}
	p["keyword"] = "k"
	p["matching_type"] = 1
	p["sort"] = -1
	p["reply_num"] = 1
	if OAReplyWriteCheck(p, false) != "排序值须大于或等于0" {
		t.Fatal(OAReplyWriteCheck(p, false))
	}
	p["sort"] = 0
	if OAReplyWriteCheck(p, true) != "参数缺失" {
		t.Fatal(OAReplyWriteCheck(p, true))
	}
	p["id"] = "abc"
	if OAReplyWriteCheck(p, true) != "参数格式错误" {
		t.Fatal(OAReplyWriteCheck(p, true))
	}
	p["id"] = 0
	if OAReplyWriteCheck(p, true) != "" {
		t.Fatal(OAReplyWriteCheck(p, true))
	}
	p["id"] = 1
	if OAReplyWriteCheck(p, true) != "" {
		t.Fatal(OAReplyWriteCheck(p, true))
	}
	if OAReplyWriteCheck(map[string]any{"reply_type": nil, "name": "r", "content_type": 1, "content": "hi", "status": 0}, false) != "请输入回复类型" {
		t.Fatal(OAReplyWriteCheck(map[string]any{"reply_type": nil, "name": "r", "content_type": 1, "content": "hi", "status": 0}, false))
	}
	nullSort := map[string]any{"reply_type": 2, "name": "r", "content_type": 1, "content": "hi", "status": 0, "keyword": "k", "matching_type": 1, "sort": nil, "reply_num": 1}
	if OAReplyWriteCheck(nullSort, false) != "请输入排序值" {
		t.Fatal(OAReplyWriteCheck(nullSort, false))
	}
	nullStatus := map[string]any{"reply_type": 1, "name": "r", "content_type": 1, "content": "hi", "status": nil}
	if OAReplyWriteCheck(nullStatus, false) != "请选择启用状态" {
		t.Fatal(OAReplyWriteCheck(nullStatus, false))
	}
}

func TestUserPasswordCheck(t *testing.T) {
	if UserPasswordCheck(map[string]any{}) != "请输入密码" {
		t.Fatal(UserPasswordCheck(map[string]any{}))
	}
	if UserPasswordCheck(map[string]any{"password": "abc12", "password_confirm": "abc12"}) != "密码须在6-25位之间" {
		t.Fatal("length")
	}
	if UserPasswordCheck(map[string]any{"password": "abcdef", "password_confirm": "abcdef"}) != "" {
		t.Fatal("PHP alphaNum allows all-letter passwords")
	}
	if UserPasswordCheck(map[string]any{"password": "123456", "password_confirm": "123456"}) != "" {
		t.Fatal("PHP alphaNum allows all-digit passwords")
	}
	if UserPasswordCheck(map[string]any{"password": "abc-def", "password_confirm": "abc-def"}) != "密码须为字母数字组合" {
		t.Fatal("symbols must fail")
	}
	if UserPasswordCheck(map[string]any{"password": "abc123"}) != "请确认密码" {
		t.Fatal("confirm required")
	}
	if UserPasswordCheck(map[string]any{"password": "abc123", "password_confirm": "abc124"}) != "两次输入的密码不一致" {
		t.Fatal("mismatch")
	}
	if UserPasswordCheck(map[string]any{"password": "abc123", "password_confirm": "abc123"}) != "" {
		t.Fatal("expected ok")
	}
}

func TestPlatformWebSettingCheck(t *testing.T) {
	if PlatformWebSettingCheck(map[string]any{}) != "请填写网站名称" {
		t.Fatal(PlatformWebSettingCheck(map[string]any{}))
	}
	long := "一二三四五六七八九十一二三四五六七八九十12345678901"
	if PlatformWebSettingCheck(map[string]any{"name": long}) != "网站名称最长为12个字符" {
		t.Fatal(PlatformWebSettingCheck(map[string]any{"name": long}))
	}
}

func TestTransactionSettingCheck(t *testing.T) {
	if TransactionSettingCheck(map[string]any{}) != "请选择系统取消待付款订单方式" {
		t.Fatal("empty")
	}
	if TransactionSettingCheck(map[string]any{"cancel_unpaid_orders": 0}) != "请选择系统自动核销订单方式" {
		t.Fatal("verification required")
	}
	if TransactionSettingCheck(map[string]any{"cancel_unpaid_orders": 1, "verification_orders": 0}) != "系统取消待付款订单时间未填写" {
		t.Fatal("times")
	}
	if TransactionSettingCheck(map[string]any{"cancel_unpaid_orders": 1, "cancel_unpaid_orders_times": 1.5, "verification_orders": 0}) != "系统取消待付款订单时间须为整型" {
		t.Fatal("float")
	}
}

func TestSmsConfigWriteCheck(t *testing.T) {
	if SmsConfigWriteCheck(map[string]any{}) != "请选择类型" {
		t.Fatal(SmsConfigWriteCheck(map[string]any{}))
	}
	if SmsConfigWriteCheck(map[string]any{"type": "ali"}) != "请输入签名" {
		t.Fatal("sign")
	}
}

func TestOAMenuCheck(t *testing.T) {
	if OAMenuCheck(nil) != "请设置正确格式菜单" {
		t.Fatal("empty")
	}
	if OAMenuCheck([]any{map[string]any{"name": "a"}, map[string]any{"name": "b"}, map[string]any{"name": "c"}, map[string]any{"name": "d"}}) != "一级菜单超出限制(最多3个)" {
		t.Fatal("count")
	}
	if OAMenuCheck([]any{map[string]any{"name": "一二三四五"}}) != "一级菜单名称字数不能超过4个汉字或8个字母" {
		t.Fatal("width")
	}
	if OAMenuCheck([]any{map[string]any{"name": "菜单", "has_menu": false}}) != "一级菜单未选择菜单类型" {
		t.Fatal("type")
	}
	if OAMenuCheck([]any{map[string]any{"name": 0, "has_menu": false, "type": "click", "key": "k"}}) != "请输入一级菜单名称" {
		t.Fatal("name0")
	}
	if OAMenuCheck([]any{map[string]any{"name": "菜单", "has_menu": false, "type": "click", "key": 0}}) != "请输入关键字" {
		t.Fatal("key0")
	}
	if OAMenuCheck([]any{map[string]any{"name": "菜单", "has_menu": false, "type": "view", "url": "0"}}) != "请输入网页链接" {
		t.Fatal("url0")
	}
}

func TestCoerceOAMenuHasMenu(t *testing.T) {
	out, _ := CoerceOAMenuHasMenu([]any{
		map[string]any{"name": "菜单", "has_menu": 1},
		map[string]any{"name": "空", "has_menu": 0},
	}).([]any)
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	first, _ := out[0].(map[string]any)
	second, _ := out[1].(map[string]any)
	if first["has_menu"] != true || second["has_menu"] != false {
		t.Fatalf("has_menu %v %v", first["has_menu"], second["has_menu"])
	}
}

func TestRechargeAPICheck(t *testing.T) {
	if RechargeAPICheck(map[string]any{}, 1, 0) != "请填写充值金额" {
		t.Fatal("money")
	}
	if RechargeAPICheck(map[string]any{"money": 0}, 1, 0) != "请填写大于0的充值金额" {
		t.Fatal("gt")
	}
	if RechargeAPICheck(map[string]any{"money": 1}, 0, 0) != "充值功能已关闭" {
		t.Fatal("closed")
	}
	if RechargeAPICheck(map[string]any{"money": 1}, 1, 10) != "最低充值金额10元" {
		t.Fatal(RechargeAPICheck(map[string]any{"money": 1}, 1, 10))
	}
}

func TestDictTypeWriteCheck(t *testing.T) {
	if DictTypeWriteCheck(map[string]any{}) != "请填写字典名称" {
		t.Fatal(DictTypeWriteCheck(map[string]any{}))
	}
	if DictTypeWriteCheck(map[string]any{"name": "n", "type": "t", "status": 1}) != "" {
		t.Fatal("expected ok")
	}
	if DictTypeWriteCheck(map[string]any{"name": "n", "type": "t", "status": 2}) != "status必须在 0,1 范围内" {
		t.Fatal(DictTypeWriteCheck(map[string]any{"name": "n", "type": "t", "status": 2}))
	}
	if msg := DictTypeWriteCheckTaken(map[string]any{"name": "n", "type": "taken"}, func(string) bool { return true }); msg != "字典类型已存在" {
		t.Fatal(msg)
	}
}

func TestUploadExtCheck(t *testing.T) {
	img := []string{"jpg", "png"}
	vid := []string{"mp4"}
	file := []string{"txt", "pdf"}
	if UploadExtCheck("image", "exe", img, vid, file) != "不允许上传exe后缀文件" {
		t.Fatal(UploadExtCheck("image", "exe", img, vid, file))
	}
	if UploadExtCheck("image", "txt", img, vid, file) != "上传图片不允许上传txt文件" {
		t.Fatal(UploadExtCheck("image", "txt", img, vid, file))
	}
	if UploadExtCheck("image", "jpg", img, vid, file) != "" {
		t.Fatal("jpg should pass")
	}
}

func TestAdminEditSelfCheck(t *testing.T) {
	if AdminEditSelfCheck(map[string]any{}) != "请填写名称" {
		t.Fatal(AdminEditSelfCheck(map[string]any{}))
	}
	if AdminEditSelfCheck(map[string]any{"name": "n"}) != "请选择头像" {
		t.Fatal("avatar")
	}
	if AdminEditSelfCheck(map[string]any{"name": "n", "avatar": "a.png", "password": "123456"}) != "请填写当前密码" {
		t.Fatal("old")
	}
}

func TestMenuRoleDeptJobsCheck(t *testing.T) {
	if MenuWriteCheck(map[string]any{}, false) != "请选择上级菜单" {
		t.Fatal(MenuWriteCheck(map[string]any{}, false))
	}
	if msg := MenuWriteCheckTaken(map[string]any{"pid": 0, "type": "M", "name": "taken"}, false, func(string, string) bool { return true }); msg != "菜单名称已存在" {
		t.Fatal(msg)
	}
	if RoleWriteCheck(map[string]any{}, false) != "请输入角色名称" {
		t.Fatal(RoleWriteCheck(map[string]any{}, false))
	}
	if msg := RoleWriteCheckTaken(map[string]any{"name": "taken", "menu_id": "x"}, false, func(string) bool { return true }); msg != "角色名称已存在" {
		t.Fatal(msg)
	}
	if DeptWriteCheck(map[string]any{}, false) != "请选择上级部门" {
		t.Fatal(DeptWriteCheck(map[string]any{}, false))
	}
	if DeptWriteCheck(map[string]any{"pid": 1, "name": "n", "status": 2}, false) != "status必须在 0,1 范围内" {
		t.Fatal(DeptWriteCheck(map[string]any{"pid": 1, "name": "n", "status": 2}, false))
	}
	if JobsWriteCheck(map[string]any{}, false) != "请填写岗位名称" {
		t.Fatal(JobsWriteCheck(map[string]any{}, false))
	}
	if msg := JobsWriteCheckTaken(map[string]any{"name": "taken"}, false, func(string) bool { return true }, nil); msg != "岗位名称已存在" {
		t.Fatal(msg)
	}
	if JobsWriteCheck(map[string]any{"name": "n", "code": "c", "status": 2}, false) != "岗位状态值错误" {
		t.Fatal(JobsWriteCheck(map[string]any{"name": "n", "code": "c", "status": 2}, false))
	}
}

func TestPayQueryCheck(t *testing.T) {
	if PayQueryCheck(map[string]any{}) != "参数缺失" {
		t.Fatal("from")
	}
	if PayQueryCheck(map[string]any{"from": "recharge"}) != "订单参数缺失" {
		t.Fatal("order")
	}
}

func TestPayPayCheck(t *testing.T) {
	if PayPayCheck(map[string]any{"from": "recharge", "order_id": 1}) != "支付方式参数缺失" {
		t.Fatal(PayPayCheck(map[string]any{"from": "recharge", "order_id": 1}))
	}
	if PayPayCheck(map[string]any{"from": "recharge", "pay_way": nil, "order_id": 1}) != "支付方式参数缺失" {
		t.Fatal(PayPayCheck(map[string]any{"from": "recharge", "pay_way": nil, "order_id": 1}))
	}
	if PayPayCheck(map[string]any{"from": "recharge", "pay_way": 9, "order_id": 1}) != "支付方式参数错误" {
		t.Fatal(PayPayCheck(map[string]any{"from": "recharge", "pay_way": 9, "order_id": 1}))
	}
	if PayPayCheck(map[string]any{"from": "recharge", "pay_way": 1, "order_id": 1}) != "" {
		t.Fatal(PayPayCheck(map[string]any{"from": "recharge", "pay_way": 1, "order_id": 1}))
	}
}

func TestDbFieldType(t *testing.T) {
	if DbFieldType("varchar(255)") != "string" {
		t.Fatal(DbFieldType("varchar(255)"))
	}
	if DbFieldType("int(11)") != "int" {
		t.Fatal(DbFieldType("int(11)"))
	}
	if DbFieldType("decimal(10,2)") != "float" {
		t.Fatal(DbFieldType("decimal(10,2)"))
	}
}

func TestParseDateTime(t *testing.T) {
	if ParseDateTime("2024-01-02 03:04:05") == 0 {
		t.Fatal("expected unix ts")
	}
	if ParseDateTime("2024-01-02") == 0 {
		t.Fatal("expected date ts")
	}
	if ParseDateTime("") != 0 {
		t.Fatal("empty should be 0")
	}
}

func TestLoginTerminalCheck(t *testing.T) {
	if LoginTerminalCheck(map[string]any{}) != "terminal不能为空" {
		t.Fatal(LoginTerminalCheck(map[string]any{}))
	}
	if LoginTerminalCheck(map[string]any{"terminal": 99}) != "terminal必须在 1,2 范围内" {
		t.Fatal(LoginTerminalCheck(map[string]any{"terminal": 99}))
	}
	if LoginTerminalCheck(map[string]any{"terminal": 1}) != "" {
		t.Fatal("terminal 1 should pass")
	}
}

func TestAuthAdminAddCheck(t *testing.T) {
	if AuthAdminAddCheck(map[string]any{}) != "账号不能为空" {
		t.Fatal(AuthAdminAddCheck(map[string]any{}))
	}
	base := map[string]any{"account": "tmp1", "name": "临时员", "password": "likeadmin", "password_confirm": "likeadmin", "multipoint_login": 1}
	if AuthAdminAddCheck(base) != "请选择角色" {
		t.Fatal(AuthAdminAddCheck(base))
	}
	base["role_id"] = []any{1.0}
	delete(base, "multipoint_login")
	if AuthAdminAddCheck(base) != "请选择是否支持多处登录" {
		t.Fatal(AuthAdminAddCheck(base))
	}
	base["multipoint_login"] = 1
	delete(base, "password_confirm")
	if AuthAdminAddCheck(base) != "确认密码不能为空" {
		t.Fatal(AuthAdminAddCheck(base))
	}
	if msg := AuthAdminAddCheckTaken(map[string]any{"account": "admin"}, func(string) bool { return true }, nil); msg != "账号已存在" {
		t.Fatal(msg)
	}
}

func TestTenantAdminAddCheckTaken(t *testing.T) {
	if TenantAdminAddCheck(map[string]any{}) != "请选择对应的租户" {
		t.Fatal(TenantAdminAddCheck(map[string]any{}))
	}
	if msg := TenantAdminAddCheckTaken(map[string]any{"tenant_id": 999999}, func(uint) bool { return false }, nil); msg != "对应租户账号不存在" {
		t.Fatal(msg)
	}
	if msg := TenantAdminAddCheckTaken(map[string]any{"tenant_id": 1, "account": "pair1"}, func(uint) bool { return true }, func(uint, string) bool { return true }); msg != "账号已存在" {
		t.Fatal(msg)
	}
}

func TestTenantAdminEditCheck(t *testing.T) {
	p := map[string]any{"id": 1, "tenant_id": 1, "name": "管理员"}
	if TenantAdminEditCheck(p) != "" {
		t.Fatal(TenantAdminEditCheck(p))
	}
	p["password"] = "123"
	p["password_confirm"] = "123"
	if TenantAdminEditCheck(p) != "密码长度须在6-32位字符" {
		t.Fatal(TenantAdminEditCheck(p))
	}
	p["password"] = "123456"
	p["password_confirm"] = "654321"
	if TenantAdminEditCheck(p) != "两次输入的密码不一致" {
		t.Fatal(TenantAdminEditCheck(p))
	}
	p["password_confirm"] = "123456"
	if TenantAdminEditCheck(p) != "" {
		t.Fatal(TenantAdminEditCheck(p))
	}
}

func TestAuthAdminEditCheck(t *testing.T) {
	p := map[string]any{"account": "pair1", "name": "超级管理员", "multipoint_login": 1}
	if AuthAdminEditCheck(p, true) != "请选择状态" {
		t.Fatal(AuthAdminEditCheck(p, true))
	}
	p["disable"] = 1
	if AuthAdminEditCheck(p, true) != "超级管理员不允许被禁用" {
		t.Fatal(AuthAdminEditCheck(p, true))
	}
	p["disable"] = 0
	if AuthAdminEditCheck(p, false) != "请选择角色" {
		t.Fatal(AuthAdminEditCheck(p, false))
	}
	if msg := AuthAdminEditCheckTaken(map[string]any{"account": "admin"}, false, func(string) bool { return true }, nil); msg != "账号已存在" {
		t.Fatal(msg)
	}
}

func TestPayWaySetCheck(t *testing.T) {
	p := map[string]any{
		"1": []any{
			map[string]any{"id": 1, "is_default": 0, "status": 1},
		},
	}
	if PayWaySetCheck(p) != "H5支付场景缺少默认支付" {
		t.Fatal(PayWaySetCheck(p))
	}
	p["1"] = []any{
		map[string]any{"id": 1, "is_default": 1, "status": 0},
		map[string]any{"id": 2, "is_default": 0, "status": 1},
	}
	if PayWaySetCheck(p) != "H5支付场景的默认支付未开启支付状态" {
		t.Fatal(PayWaySetCheck(p))
	}
}

func TestUserSetInfoCheck(t *testing.T) {
	if UserSetInfoCheck(map[string]any{}) != "参数缺失" {
		t.Fatal(UserSetInfoCheck(map[string]any{}))
	}
	if UserSetInfoCheck(map[string]any{"field": "nickname"}) != "值不存在" {
		t.Fatal(UserSetInfoCheck(map[string]any{"field": "nickname"}))
	}
	if UserSetInfoCheck(map[string]any{"field": "nickname", "value": "n"}) != "" {
		t.Fatal("expected ok")
	}
}

func TestOAReplySortCheck(t *testing.T) {
	if OAReplySortCheck(map[string]any{}) != "请输入新排序值" {
		t.Fatal(OAReplySortCheck(map[string]any{}))
	}
	if OAReplySortCheck(map[string]any{"new_sort": 1.5}) != "新排序值须为整型" {
		t.Fatal(OAReplySortCheck(map[string]any{"new_sort": 1.5}))
	}
	if OAReplySortCheck(map[string]any{"new_sort": -1}) != "新排序值须大于或等于0" {
		t.Fatal(OAReplySortCheck(map[string]any{"new_sort": -1}))
	}
}

func TestArticleCateShowCheck(t *testing.T) {
	if ArticleCateShowCheck(map[string]any{}) != "is_show不能为空" {
		t.Fatal(ArticleCateShowCheck(map[string]any{}))
	}
	if ArticleCateShowCheck(map[string]any{"is_show": 2}) != "is_show必须在 0,1 范围内" {
		t.Fatal(ArticleCateShowCheck(map[string]any{"is_show": 2}))
	}
	if ArticleCateShowCheck(map[string]any{"is_show": 0}) != "" {
		t.Fatal("is_show 0 should pass")
	}
}

func TestSexChannelMoney(t *testing.T) {
	if SexDesc(0) != "未知" || SexDesc(1) != "男" || SexDesc(2) != "女" {
		t.Fatal(SexDesc(0), SexDesc(1), SexDesc(2))
	}
	if ChannelDesc(1) != "微信小程序" || ChannelDesc(4) != "电脑PC" {
		t.Fatal(ChannelDesc(1), ChannelDesc(4))
	}
	if MoneyString(0) != "0.00" || MoneyString(1.5) != "1.50" {
		t.Fatal(MoneyString(0), MoneyString(1.5))
	}
	if EmptyToNil("") != nil || EmptyToNil("tx") != "tx" {
		t.Fatal(EmptyToNil(""), EmptyToNil("tx"))
	}
}

func TestUpgradeCheck(t *testing.T) {
	if UpgradeCheck(map[string]any{}) != "参数缺失" {
		t.Fatal(UpgradeCheck(map[string]any{}))
	}
	if UpgradeCheck(map[string]any{"id": 1}) != "参数缺失" {
		t.Fatal(UpgradeCheck(map[string]any{"id": 1}))
	}
	if UpgradeCheck(map[string]any{"id": 1, "update_type": 2}) != "更新类型错误" {
		t.Fatal(UpgradeCheck(map[string]any{"id": 1, "update_type": 2}))
	}
	if UpgradeCheck(map[string]any{"id": 1, "update_type": 1}) != "" {
		t.Fatal("valid upgrade params should pass field checks")
	}
	if UpgradeDownloadCheck(map[string]any{}) != "参数缺失" {
		t.Fatal(UpgradeDownloadCheck(map[string]any{}))
	}
}

func TestCopyrightConfigCheck(t *testing.T) {
	if CopyrightConfigCheck(nil) != "参数异常" {
		t.Fatal("nil")
	}
	if CopyrightConfigCheck("bad") != "参数异常" {
		t.Fatal("string")
	}
	if CopyrightConfigCheck(1) != "参数异常" {
		t.Fatal("number")
	}
	if CopyrightConfigCheck([]any{}) != "" {
		t.Fatal("empty array")
	}
	if CopyrightConfigCheck([]any{map[string]any{"name": "x"}}) != "" {
		t.Fatal("array")
	}
	if CopyrightConfigCheck(map[string]any{"0": "x"}) != "" {
		t.Fatal("assoc array")
	}
}

func TestWebScanLoginCheck(t *testing.T) {
	if WebScanLoginCheck(map[string]any{}) != "参数缺失" {
		t.Fatal(WebScanLoginCheck(map[string]any{}))
	}
	if WebScanLoginCheck(map[string]any{"code": "x"}) != "昵称缺少" {
		t.Fatal(WebScanLoginCheck(map[string]any{"code": "x"}))
	}
	if WebScanLoginCheck(map[string]any{"code": "x", "state": "s"}) != "" {
		t.Fatal("valid")
	}
}

func TestUintSlicesChanged(t *testing.T) {
	if UintSlicesChanged([]uint{1, 2}, []uint{1, 2}) {
		t.Fatal("same order should be unchanged")
	}
	if !UintSlicesChanged([]uint{1, 2}, []uint{2, 1}) {
		t.Fatal("reordered roles should count as changed")
	}
	if !UintSlicesChanged([]uint{1}, []uint{1, 2}) {
		t.Fatal("added role should count as changed")
	}
	if !UintSlicesChanged([]uint{1, 2}, []uint{1}) {
		t.Fatal("removed role should count as changed")
	}
	if !UintSlicesChanged([]uint{1}, []uint{2}) {
		t.Fatal("replaced role should count as changed")
	}
	if UintSlicesChanged(nil, nil) {
		t.Fatal("empty should be unchanged")
	}
}

func TestWechatJsConfigCheck(t *testing.T) {
	if WechatJsConfigCheck(map[string]any{}) != "请提供url" {
		t.Fatal(WechatJsConfigCheck(map[string]any{}))
	}
	if WechatJsConfigCheck(map[string]any{"url": ""}) != "请提供url" {
		t.Fatal(WechatJsConfigCheck(map[string]any{"url": ""}))
	}
	if WechatJsConfigCheck(map[string]any{"url": "https://example.com/"}) != "" {
		t.Fatal("valid")
	}
}
