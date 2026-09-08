package util

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var registerAccountChars = regexp.MustCompile(`^[A-Za-z0-9]+$`)

// ValidRegisterAccount mirrors PHP RegisterValidate account rules.
func ValidRegisterAccount(account string) string {
	if account == "" {
		return "请输入账号"
	}
	if n := len(account); n < 3 || n > 12 {
		return "账号须为3-12位之间"
	}
	if !registerAccountChars.MatchString(account) {
		return "账号须为字母数字组合"
	}
	hasLetter, hasDigit := false, false
	for _, r := range account {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return "账号须为字母数字组合"
	}
	return ""
}

// ValidRegisterPassword mirrors PHP RegisterValidate password rules.
func ValidRegisterPassword(password string) string {
	if password == "" {
		return "请输入密码"
	}
	if n := utf8.RuneCountInString(password); n < 6 || n > 20 {
		return "密码须在6-25位之间"
	}
	var lower, upper, digit, special int
	for _, r := range password {
		switch {
		case r >= 'a' && r <= 'z':
			lower++
		case r >= 'A' && r <= 'Z':
			upper++
		case r >= '0' && r <= '9':
			digit++
		default:
			special++
		}
	}
	n := utf8.RuneCountInString(password)
	if (lower == n) || (upper == n) || (digit == n) || (special == n) {
		return "密码须为数字,字母或符号组合"
	}
	return ""
}

func AdminWriteCheck(account, name, password string, needPwd bool) string {
	if account == "" {
		return "账号不能为空"
	}
	if n := len([]rune(account)); n < 1 || n > 32 {
		return "账号长度须在1-32位字符"
	}
	if name == "" {
		return "名称不能为空"
	}
	if n := len([]rune(name)); n < 1 || n > 16 {
		return "名称须在1-16位字符"
	}
	if needPwd {
		if password == "" {
			return "密码不能为空"
		}
		if n := len(password); n < 6 || n > 32 {
			return "密码长度须在6-32位字符"
		}
	} else if password != "" && (len(password) < 6 || len(password) > 32) {
		return "密码长度须在6-32位字符"
	}
	return ""
}

var chinaMobile = regexp.MustCompile(`^1[3-9]\d{9}$`)

// ValidChinaMobile mirrors ThinkPHP mobile rule used by UserValidate.
func ValidChinaMobile(mobile string) string {
	if mobile == "" {
		return "请输入内容"
	}
	if !chinaMobile.MatchString(mobile) {
		return "手机号码格式错误"
	}
	return ""
}

func FileNameCheck(name string) string {
	// ThinkPHP FileValidate name.require: only exact "" fails; whitespace passes.
	if name == "" {
		return "请填写分组名称"
	}
	if n := len([]rune(name)); n > 20 {
		return "分组名称长度须为20字符内"
	}
	return ""
}

func FileMoveCheck(p map[string]any, ids []uint) string {
	// PHP FileValidate $rule lists cid before ids: require|number then require|array.
	if !phpRequired(p, "cid") {
		return "缺少cid参数"
	}
	if !isPHPNumber(p["cid"]) {
		return "cid必须是数字"
	}
	if !phpRequired(p, "ids") {
		return "缺少ids参数"
	}
	if !isArrayValue(p["ids"]) {
		return "ids必须是数组"
	}
	if len(ids) == 0 {
		return "缺少ids参数"
	}
	return ""
}

func FileDeleteCheck(p map[string]any, ids []uint) string {
	if !phpRequired(p, "ids") {
		return "缺少ids参数"
	}
	if !isArrayValue(p["ids"]) {
		return "ids必须是数组"
	}
	if len(ids) == 0 {
		return "缺少ids参数"
	}
	return ""
}

func FileIDCheck(p map[string]any) string {
	if !phpRequired(p, "id") {
		return "缺少id参数"
	}
	if !isPHPNumber(p["id"]) {
		return "id必须是数字"
	}
	return ""
}

func FileAddCateCheck(p map[string]any) string {
	if !phpRequired(p, "type") {
		return "缺少type参数"
	}
	typ := ToInt(p["type"])
	if typ != 10 && typ != 20 && typ != 30 {
		return "type必须在 10,20,30 范围内"
	}
	if !phpRequired(p, "pid") {
		return "缺少pid参数"
	}
	if !isPHPNumber(p["pid"]) {
		return "pid必须是数字"
	}
	if !phpRequired(p, "name") {
		return "请填写分组名称"
	}
	return FileNameCheck(ToString(p["name"]))
}

func FileEditCateCheck(p map[string]any) string {
	if msg := FileIDCheck(p); msg != "" {
		return msg
	}
	if !phpRequired(p, "name") {
		return "请填写分组名称"
	}
	return FileNameCheck(ToString(p["name"]))
}

func UserPasswordCheck(p map[string]any) string {
	pwd := strings.TrimSpace(ToString(p["password"]))
	if pwd == "" {
		return "请输入密码"
	}
	if n := len(pwd); n < 6 || n > 20 {
		return "密码须在6-25位之间"
	}
	// PHP PasswordValidate is length:6,20|alphaNum (ctype_alnum).
	// All-letter / all-digit passwords are valid; symbols are not.
	for _, r := range pwd {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "密码须为字母数字组合"
		}
	}
	if !phpRequired(p, "password_confirm") {
		return "请确认密码"
	}
	if pwd != strings.TrimSpace(ToString(p["password_confirm"])) {
		return "两次输入的密码不一致"
	}
	return ""
}

func FileRenameCheck(p map[string]any) string {
	return FileEditCateCheck(p)
}

func OAReplyIDCheck(p map[string]any) string {
	if !phpRequired(p, "id") {
		return "参数缺失"
	}
	if !isWholeNumber(p["id"]) {
		return "参数格式错误"
	}
	return ""
}

func OAReplyWriteCheck(p map[string]any, needID bool) string {
	if needID {
		if msg := OAReplyIDCheck(p); msg != "" {
			return msg
		}
	}
	if !phpRequired(p, "reply_type") {
		return "请输入回复类型"
	}
	rt := ToInt(p["reply_type"])
	if rt != 1 && rt != 2 && rt != 3 {
		return "回复类型状态值错误"
	}
	if !phpRequired(p, "name") {
		return "请输入规则名称"
	}
	if !phpRequired(p, "content_type") {
		return "请选择内容类型"
	}
	if ToInt(p["content_type"]) != 1 {
		return "内容类型状态值有误"
	}
	if !phpRequired(p, "content") {
		return "请输入回复内容"
	}
	if !phpRequired(p, "status") {
		return "请选择启用状态"
	}
	st := ToInt(p["status"])
	if st != 0 && st != 1 {
		return "启用状态值错误"
	}
	if rt == 2 {
		if !phpRequired(p, "keyword") {
			return "请输入关键词"
		}
		if !phpRequired(p, "matching_type") {
			return "请选择匹配类型"
		}
		mt := ToInt(p["matching_type"])
		if mt != 1 && mt != 2 {
			return "匹配类型状态值错误"
		}
		if !phpRequired(p, "sort") {
			return "请输入排序值"
		}
		if ToInt(p["sort"]) < 0 {
			return "排序值须大于或等于0"
		}
		if !phpRequired(p, "reply_num") {
			return "请选择回复数量"
		}
		if ToInt(p["reply_num"]) != 1 {
			return "回复数量状态值错误"
		}
	}
	return ""
}

func DictTypeWriteCheck(p map[string]any) string {
	return DictTypeWriteCheckTaken(p, nil)
}

func DictTypeWriteCheckTaken(p map[string]any, typeTaken func(string) bool) string {
	if !phpRequired(p, "name") {
		return "请填写字典名称"
	}
	if n := len([]rune(ToString(p["name"]))); n > 255 {
		return "字典名称长度须在1~255位字符"
	}
	if !phpRequired(p, "type") {
		return "请填写字典类型"
	}
	if typeTaken != nil && typeTaken(ToString(p["type"])) {
		return "字典类型已存在"
	}
	if !phpRequired(p, "status") {
		return "请选择状态"
	}
	if !inZeroOne(p["status"]) {
		return "status必须在 0,1 范围内"
	}
	if n := len([]rune(ToString(p["remark"]))); n > 200 {
		return "备注长度不能超过200"
	}
	return ""
}

func DictDataWriteCheck(p map[string]any, needTypeID bool) string {
	if !phpRequired(p, "name") {
		return "请填写字典数据名称"
	}
	if n := len([]rune(ToString(p["name"]))); n > 255 {
		return "字典数据名称长度须在1-255位字符"
	}
	if !phpRequired(p, "value") {
		return "请填写字典数据值"
	}
	if needTypeID {
		if !phpRequired(p, "type_id") {
			return "字典类型缺失"
		}
	}
	if !phpRequired(p, "status") {
		return "请选择字典数据状态"
	}
	st := ToInt(p["status"])
	if st != 0 && st != 1 {
		return "字典数据状态参数错误"
	}
	return ""
}

// phpLooseEmpty matches PHP empty() for OA menu scalars: missing, "", 0, "0".
func phpLooseEmpty(v any) bool {
	if v == nil {
		return true
	}
	s := strings.TrimSpace(ToString(v))
	return s == "" || s == "0"
}

func PHPRequired(p map[string]any, key string) bool {
	return phpRequired(p, key)
}

// PHPIsset mirrors PHP isset(): missing or JSON null is absent; "" / 0 / false stay set.
func PHPIsset(p map[string]any, key string) bool {
	v, ok := p[key]
	return ok && v != nil
}

func phpRequired(p map[string]any, key string) bool {
	v, ok := p[key]
	if !ok || v == nil {
		return false
	}
	// ThinkPHP require is `!empty($value) || '0' == $value`:
	// fail missing/null/""/[]; pass 0/"0"/false/whitespace.
	switch t := v.(type) {
	case string:
		return t != ""
	case []any:
		return len(t) > 0
	case []string:
		return len(t) > 0
	case []int:
		return len(t) > 0
	case []uint:
		return len(t) > 0
	case bool:
		return true
	default:
		return ToString(v) != ""
	}
}

func isWholeNumber(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return t == float64(int64(t))
	case float32:
		return t == float32(int32(t))
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return false
		}
		if _, err := strconv.Atoi(s); err == nil {
			return true
		}
		return false
	default:
		return false
	}
}

// isPHPNumber mirrors ThinkPHP Validate `number` = ctype_digit((string)$value).
func isPHPNumber(v any) bool {
	if v == nil {
		return false
	}
	switch t := v.(type) {
	case uint, uint8, uint16, uint32, uint64:
		return true
	case int:
		return t >= 0
	case int8:
		return t >= 0
	case int16:
		return t >= 0
	case int32:
		return t >= 0
	case int64:
		return t >= 0
	case float64:
		return t >= 0 && t == float64(int64(t))
	case float32:
		return t >= 0 && t == float32(int32(t))
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return false
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	default:
		return isPHPNumber(ToString(v))
	}
}

func InZeroOne(v any) bool {
	return inZeroOne(v)
}

func inZeroOne(v any) bool {
	n := ToInt(v)
	if n != 0 && n != 1 {
		return false
	}
	if s, ok := v.(string); ok && strings.TrimSpace(s) != "0" && strings.TrimSpace(s) != "1" {
		return false
	}
	return true
}

func isArrayValue(v any) bool {
	switch v.(type) {
	case []any, []string, []int, []int64, []float64:
		return true
	default:
		return false
	}
}

func PlatformWebSettingCheck(p map[string]any) string {
	if !phpRequired(p, "name") {
		return "请填写网站名称"
	}
	if n := len([]rune(ToString(p["name"]))); n > 30 {
		return "网站名称最长为12个字符"
	}
	if !phpRequired(p, "web_favicon") {
		return "请上传网站图标"
	}
	if !phpRequired(p, "web_logo_light") {
		return "请上传网站亮色主题logo"
	}
	if !phpRequired(p, "web_logo_dark") {
		return "请上传网站暗色主题logo"
	}
	if !phpRequired(p, "login_image") {
		return "请上传登录页广告图"
	}
	return ""
}

func TenantWebSettingCheck(p map[string]any) string {
	if !phpRequired(p, "name") {
		return "请填写网站名称"
	}
	if n := len([]rune(ToString(p["name"]))); n > 30 {
		return "网站名称最长为12个字符"
	}
	if !phpRequired(p, "web_favicon") {
		return "请上传网站图标"
	}
	if !phpRequired(p, "web_logo") {
		return "请上传网站logo"
	}
	if !phpRequired(p, "login_image") {
		return "请上传登录页广告图"
	}
	if !phpRequired(p, "shop_name") {
		return "请填写前台名称"
	}
	if !phpRequired(p, "shop_logo") {
		return "请上传前台logo"
	}
	if !phpRequired(p, "pc_logo") {
		return "请上传PC端logo"
	}
	return ""
}

func UserAvatarCheck(p map[string]any) string {
	if !phpRequired(p, "default_avatar") {
		return "请上传用户默认头像"
	}
	return ""
}

func UserRegisterConfigCheck(p map[string]any) string {
	// PHP requireIf:scene,register looks at the body field `scene`, not the
	// validation scene name used by goCheck('register').
	if ToString(p["scene"]) == "register" {
		if !phpRequired(p, "login_way") {
			return "请选择登录方式"
		}
		if !phpRequired(p, "coerce_mobile") {
			return "请选择注册强制绑定手机"
		}
	}
	if PHPIsset(p, "login_way") && !isArrayValue(p["login_way"]) {
		return "登录方式值错误"
	}
	if PHPIsset(p, "coerce_mobile") && !inZeroOne(p["coerce_mobile"]) {
		return "注册强制绑定手机值错误"
	}
	if PHPIsset(p, "login_agreement") && !inZeroOne(p["login_agreement"]) {
		return "政策协议值错误"
	}
	if PHPIsset(p, "third_auth") && !inZeroOne(p["third_auth"]) {
		return "第三方登录值错误"
	}
	if PHPIsset(p, "wechat_auth") && !inZeroOne(p["wechat_auth"]) {
		return "公众号微信授权登录值错误"
	}
	return ""
}

func TransactionSettingCheck(p map[string]any) string {
	if !phpRequired(p, "cancel_unpaid_orders") {
		return "请选择系统取消待付款订单方式"
	}
	if !inZeroOne(p["cancel_unpaid_orders"]) {
		return "系统取消待付款订单状态值有误"
	}
	if ToInt(p["cancel_unpaid_orders"]) == 1 {
		if !phpRequired(p, "cancel_unpaid_orders_times") {
			return "系统取消待付款订单时间未填写"
		}
		if !isWholeNumber(p["cancel_unpaid_orders_times"]) {
			return "系统取消待付款订单时间须为整型"
		}
		if ToInt(p["cancel_unpaid_orders_times"]) <= 0 {
			return "系统取消待付款订单时间须大于0"
		}
	}
	if !phpRequired(p, "verification_orders") {
		return "请选择系统自动核销订单方式"
	}
	if !inZeroOne(p["verification_orders"]) {
		return "系统自动核销订单状态值有误"
	}
	if ToInt(p["verification_orders"]) == 1 {
		if !phpRequired(p, "verification_orders_times") {
			return "系统自动核销订单时间未填写"
		}
		if !isWholeNumber(p["verification_orders_times"]) {
			return "系统自动核销订单时间须为整型"
		}
		if ToInt(p["verification_orders_times"]) <= 0 {
			return "系统自动核销订单时间须大于0"
		}
	}
	return ""
}

func SmsConfigWriteCheck(p map[string]any) string {
	if !phpRequired(p, "type") {
		return "请选择类型"
	}
	if !phpRequired(p, "sign") {
		return "请输入签名"
	}
	typ := ToString(p["type"])
	if typ == "tencent" && !phpRequired(p, "app_id") {
		return "请输入app_id"
	}
	if typ == "ali" && !phpRequired(p, "app_key") {
		return "请输入app_key"
	}
	if typ == "tencent" && !phpRequired(p, "secret_id") {
		return "请输入secret_id"
	}
	if !phpRequired(p, "secret_key") {
		return "请输入secret_key"
	}
	if !phpRequired(p, "status") {
		return "请选择状态"
	}
	return ""
}

func TenantAdminEditCheck(p map[string]any) string {
	if !phpRequired(p, "id") {
		return "请选择用户"
	}
	if !phpRequired(p, "tenant_id") {
		return "请选择对应的租户"
	}
	if !phpRequired(p, "name") {
		return "请输入用户名"
	}
	if PHPIsset(p, "password") && strings.TrimSpace(ToString(p["password"])) != "" {
		if n := len(ToString(p["password"])); n < 6 || n > 32 {
			return "密码长度须在6-32位字符"
		}
		if !phpRequired(p, "password_confirm") {
			return "确认密码不能为空"
		}
		if ToString(p["password"]) != ToString(p["password_confirm"]) {
			return "两次输入的密码不一致"
		}
	}
	return ""
}

func TenantAdminAddCheck(p map[string]any) string {
	return TenantAdminAddCheckTaken(p, nil, nil)
}

func TenantAdminAddCheckTaken(p map[string]any, tenantExists func(uint) bool, accountTaken func(tid uint, account string) bool) string {
	if !phpRequired(p, "tenant_id") {
		return "请选择对应的租户"
	}
	if tenantExists != nil && !tenantExists(uint(ToInt(p["tenant_id"]))) {
		return "对应租户账号不存在"
	}
	if !phpRequired(p, "account") {
		return "请输入账户"
	}
	if n := len([]rune(ToString(p["account"]))); n < 1 || n > 32 {
		return "账号长度须在1-32位字符"
	}
	if accountTaken != nil && accountTaken(uint(ToInt(p["tenant_id"])), ToString(p["account"])) {
		return "账号已存在"
	}
	if !phpRequired(p, "name") {
		return "请输入用户名"
	}
	if !phpRequired(p, "password") {
		return "密码不能为空"
	}
	if n := len(ToString(p["password"])); n < 6 || n > 32 {
		return "密码长度须在6-32位字符"
	}
	if !phpRequired(p, "password_confirm") {
		return "确认密码不能为空"
	}
	if ToString(p["password"]) != ToString(p["password_confirm"]) {
		return "两次输入的密码不一致"
	}
	return ""
}

func ChannelOASetCheck(p map[string]any) string {
	if !phpRequired(p, "app_id") {
		return "请填写AppID"
	}
	if !phpRequired(p, "app_secret") {
		return "请填写AppSecret"
	}
	if !phpRequired(p, "encryption_type") {
		return "请选择消息加密方式"
	}
	et := ToInt(p["encryption_type"])
	if et != 1 && et != 2 && et != 3 {
		return "消息加密方式状态值错误"
	}
	return ""
}

func ChannelMnpSetCheck(p map[string]any) string {
	if !phpRequired(p, "app_id") {
		return "请填写AppID"
	}
	if !phpRequired(p, "app_secret") {
		return "请填写AppSecret"
	}
	return ""
}

func ChannelOpenSetCheck(p map[string]any) string {
	if !phpRequired(p, "app_id") {
		return "请输入appId"
	}
	if !phpRequired(p, "app_secret") {
		return "请输入appSecret"
	}
	return ""
}

func ChannelH5SetCheck(p map[string]any) string {
	if !phpRequired(p, "status") {
		return "请选择启用状态"
	}
	if !inZeroOne(p["status"]) {
		return "启用状态值有误"
	}
	return ""
}

func RechargeAPICheck(p map[string]any, status int, minAmount float64) string {
	if !phpRequired(p, "money") {
		return "请填写充值金额"
	}
	money := ToFloat(p["money"])
	if money <= 0 {
		return "请填写大于0的充值金额"
	}
	if status != 1 {
		return "充值功能已关闭"
	}
	if money < minAmount {
		return "最低充值金额" + ToString(FormatAmount(minAmount)) + "元"
	}
	return ""
}

func GeneratorEditCheck(p map[string]any) string {
	if !phpRequired(p, "id") {
		return "表id缺失"
	}
	// Remaining field rules run only after the caller confirms the row exists
	// (PHP EditTableValidate checkTableData is on id, before table_name).
	return GeneratorEditFields(p)
}

func GeneratorEditFields(p map[string]any) string {
	if !phpRequired(p, "table_name") {
		return "请填写表名称"
	}
	if !phpRequired(p, "table_comment") {
		return "请填写表描述"
	}
	if !phpRequired(p, "template_type") {
		return "请选择模板类型"
	}
	if ToInt(p["template_type"]) != 0 && ToInt(p["template_type"]) != 1 {
		return "模板类型参数错误"
	}
	if !phpRequired(p, "generate_type") {
		return "请选择生成方式"
	}
	if ToInt(p["generate_type"]) != 0 && ToInt(p["generate_type"]) != 1 {
		return "生成方式类型错误"
	}
	if !phpRequired(p, "module_name") {
		return "请填写模块名称"
	}
	cols, ok := p["table_column"]
	if !ok || cols == nil {
		return "表字段信息缺失"
	}
	arr, isArr := cols.([]any)
	if !isArr {
		return "表字段信息类型错误"
	}
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			return "表字段id参数缺失"
		}
			if !PHPIsset(m, "id") {
			return "表字段id参数缺失"
		}
		if !PHPIsset(m, "query_type") {
			return "请选择查询方式"
		}
		if !PHPIsset(m, "view_type") {
			return "请选择显示类型"
		}
	}
	return ""
}

func MBStrWidth(s string) int {
	w := 0
	for _, r := range s {
		if r <= 127 {
			w++
		} else {
			w += 2
		}
	}
	return w
}

func phpLooseTrue(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(t)
		return s != "" && s != "0" && s != "false"
	default:
		return ToInt(v) != 0
	}
}

// CoerceOAMenuHasMenu mirrors OfficialAccountMenuLogic::detail: has_menu is a bool.
func CoerceOAMenuHasMenu(data any) any {
	arr, ok := data.([]any)
	if !ok {
		return data
	}
	out := make([]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok || m == nil {
			out = append(out, item)
			continue
		}
		cp := make(map[string]any, len(m)+1)
		for k, v := range m {
			cp[k] = v
		}
		cp["has_menu"] = phpLooseTrue(cp["has_menu"])
		out = append(out, cp)
	}
	return out
}

func OAMenuCheck(menu []any) string {
	if len(menu) == 0 {
		return "请设置正确格式菜单"
	}
	if len(menu) > 3 {
		return "一级菜单超出限制(最多3个)"
	}
	for _, item := range menu {
		m, ok := item.(map[string]any)
		if !ok || m == nil {
			return "一级菜单项须为数组格式"
		}
		name := strings.TrimSpace(ToString(m["name"]))
		// PHP OfficialAccountMenuLogic uses empty(), so 0/"0" is absent.
		if phpLooseEmpty(m["name"]) {
			return "请输入一级菜单名称"
		}
		if MBStrWidth(name) > 8 {
			return "一级菜单名称字数不能超过4个汉字或8个字母"
		}
		hasMenu := phpLooseTrue(m["has_menu"])
		if !hasMenu {
			if !phpRequired(m, "type") {
				return "一级菜单未选择菜单类型"
			}
			if !InFold([]string{"click", "view", "miniprogram"}, ToString(m["type"])) {
				return "一级菜单类型错误"
			}
			if msg := oaMenuTypeCheck(m); msg != "" {
				return msg
			}
		}
		if hasMenu {
			sub, _ := m["sub_button"].([]any)
			if len(sub) == 0 {
				return "请配置子菜单"
			}
		}
		if sub, ok := m["sub_button"].([]any); ok && len(sub) > 0 {
			if msg := oaMenuSubCheck(sub); msg != "" {
				return msg
			}
		}
	}
	return ""
}

func oaMenuSubCheck(sub []any) string {
	if len(sub) > 5 {
		return "二级菜单超出限制(最多5个)"
	}
	for _, item := range sub {
		m, ok := item.(map[string]any)
		if !ok || m == nil {
			return "二级菜单项须为数组"
		}
		name := strings.TrimSpace(ToString(m["name"]))
		if phpLooseEmpty(m["name"]) {
			return "请输入二级菜单名称"
		}
		if n := len([]rune(name)); n > 8 {
			return "二级菜单名称字数不能超过8个字符"
		}
		if !phpRequired(m, "type") || !InFold([]string{"click", "view", "miniprogram"}, ToString(m["type"])) {
			return "二级未选择菜单类型或菜单类型错误"
		}
		if msg := oaMenuTypeCheck(m); msg != "" {
			return msg
		}
	}
	return ""
}

func oaMenuTypeCheck(item map[string]any) string {
	switch ToString(item["type"]) {
	case "click":
		if phpLooseEmpty(item["key"]) {
			return "请输入关键字"
		}
	case "view":
		if phpLooseEmpty(item["url"]) {
			return "请输入网页链接"
		}
	case "miniprogram":
		if phpLooseEmpty(item["url"]) {
			return "请输入网页链接"
		}
		if phpLooseEmpty(item["appid"]) {
			return "请输入appid"
		}
		if phpLooseEmpty(item["pagepath"]) {
			return "请输入小程序路径"
		}
	}
	return ""
}

func PayWayText(way int) string {
	switch way {
	case 1:
		return "余额支付"
	case 2:
		return "微信支付"
	case 3:
		return "支付宝支付"
	default:
		return ""
	}
}

func RefundTypeText(t int) string {
	if t == 1 {
		return "后台退款"
	}
	return ""
}

func RefundStatusText(status int) string {
	switch status {
	case 0:
		return "退款中"
	case 1:
		return "退款成功"
	case 2:
		return "退款失败"
	default:
		return ""
	}
}

func RefundWayText(way int) string {
	switch way {
	case 1:
		return "线上退款"
	case 2:
		return "线下退款"
	default:
		return ""
	}
}

func PayStatusText(status int) string {
	switch status {
	case 1:
		return "已支付"
	default:
		return "未支付"
	}
}

func LoginWayAllows(raw any, scene int) bool {
	if scene == 0 {
		return false
	}
	switch t := raw.(type) {
	case []any:
		for _, item := range t {
			if ToInt(item) == scene {
				return true
			}
		}
		return false
	case []string:
		for _, item := range t {
			if ToInt(item) == scene {
				return true
			}
		}
		return false
	default:
		return ToInt(raw) == scene
	}
}

func UploadExtCheck(scene, ext string, images, videos, files []string) string {
	ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
	in := func(list []string) bool {
		for _, item := range list {
			if strings.EqualFold(strings.TrimSpace(item), ext) {
				return true
			}
		}
		return false
	}
	all := append(append(append([]string{}, images...), videos...), files...)
	if !in(all) {
		return "不允许上传" + ext + "后缀文件"
	}
	switch scene {
	case "image":
		if !in(images) {
			return "上传图片不允许上传" + ext + "文件"
		}
	case "video":
		if !in(videos) {
			return "上传视频不允许上传" + ext + "文件"
		}
	default:
		if !in(files) {
			return "上传文件不允许上传" + ext + "文件"
		}
	}
	return ""
}

func AdminEditSelfCheck(p map[string]any) string {
	if !phpRequired(p, "name") {
		return "请填写名称"
	}
	if n := len([]rune(ToString(p["name"]))); n < 1 || n > 16 {
		return "名称须在1-16位字符"
	}
	if !phpRequired(p, "avatar") {
		return "请选择头像"
	}
	if !phpRequired(p, "password") {
		return ""
	}
	pwd := ToString(p["password"])
	if n := len(pwd); n < 6 || n > 32 {
		return "密码长度须在6-32位字符"
	}
	if !phpRequired(p, "password_old") {
		return "请填写当前密码"
	}
	if !phpRequired(p, "password_confirm") {
		return "确认密码不能为空"
	}
	if ToString(p["password"]) != ToString(p["password_confirm"]) {
		return "两次输入的密码不一致"
	}
	return ""
}

func MenuWriteCheck(p map[string]any, needID bool) string {
	return MenuWriteCheckTaken(p, needID, nil)
}

func MenuWriteCheckTaken(p map[string]any, needID bool, nameTaken func(typ, name string) bool) string {
	if needID && !phpRequired(p, "id") {
		return "参数缺失"
	}
	if !phpRequired(p, "pid") {
		return "请选择上级菜单"
	}
	if !phpRequired(p, "type") {
		return "请选择菜单类型"
	}
	typ := ToString(p["type"])
	if typ != "M" && typ != "C" && typ != "A" {
		return "菜单类型参数值错误"
	}
	if !phpRequired(p, "name") {
		return "请填写菜单名称"
	}
	if n := len([]rune(ToString(p["name"]))); n < 1 || n > 30 {
		return "菜单名称长度需为1~30个字符"
	}
	if nameTaken != nil && nameTaken(typ, ToString(p["name"])) {
		return "菜单名称已存在"
	}
	if n := len([]rune(ToString(p["icon"]))); n > 100 {
		return "图标名称不能超过100个字符"
	}
	if !phpRequired(p, "sort") {
		return "请填写排序"
	}
	if ToInt(p["sort"]) < 0 {
		return "排序值需大于或等于0"
	}
	if n := len([]rune(ToString(p["perms"]))); n > 100 {
		return "权限字符不能超过100个字符"
	}
	if n := len([]rune(ToString(p["paths"]))); n > 200 {
		return "路由地址不能超过200个字符"
	}
	if n := len([]rune(ToString(p["component"]))); n > 200 {
		return "组件路径不能超过200个字符"
	}
	if n := len([]rune(ToString(p["selected"]))); n > 200 {
		return "选中菜单路径不能超过200个字符"
	}
	if n := len([]rune(ToString(p["params"]))); n > 200 {
		return "路由参数不能超过200个字符"
	}
	if !phpRequired(p, "is_cache") {
		return "请选择缓存状态"
	}
	if !inZeroOne(p["is_cache"]) {
		return "缓存状态参数值错误"
	}
	if !phpRequired(p, "is_show") {
		return "请选择显示状态"
	}
	if !inZeroOne(p["is_show"]) {
		return "显示状态参数值错误"
	}
	if !phpRequired(p, "is_disable") {
		return "请选择菜单状态"
	}
	if !inZeroOne(p["is_disable"]) {
		return "菜单状态参数值错误"
	}
	return ""
}

func RoleWriteCheck(p map[string]any, needID bool) string {
	return RoleWriteCheckTaken(p, needID, nil)
}

func RoleWriteCheckTaken(p map[string]any, needID bool, nameTaken func(string) bool) string {
	if needID && !phpRequired(p, "id") {
		return "请选择角色"
	}
	if !phpRequired(p, "name") {
		return "请输入角色名称"
	}
	if n := len([]rune(ToString(p["name"]))); n > 64 {
		return "角色名称最长为16个字符"
	}
	if nameTaken != nil && nameTaken(ToString(p["name"])) {
		return "角色名称已存在"
	}
	if v, ok := p["menu_id"]; ok && v != nil && !isArrayValue(v) {
		return "权限格式错误"
	}
	return ""
}

func DeptPidCheck(p map[string]any) string {
	if !phpRequired(p, "pid") {
		return "请选择上级部门"
	}
	if !isWholeNumber(p["pid"]) {
		return "上级部门参数错误"
	}
	return ""
}

func DeptWriteCheck(p map[string]any, needID bool) string {
	return DeptWriteCheckTaken(p, needID, nil)
}

func DeptWriteCheckTaken(p map[string]any, needID bool, nameTaken func(string) bool) string {
	if needID && !phpRequired(p, "id") {
		return "参数缺失"
	}
	if !phpRequired(p, "pid") {
		return "请选择上级部门"
	}
	if !isWholeNumber(p["pid"]) {
		return "上级部门参数错误"
	}
	if !phpRequired(p, "name") {
		return "请填写部门名称"
	}
	if nameTaken != nil && nameTaken(ToString(p["name"])) {
		return "部门名称已存在"
	}
	if n := len([]rune(ToString(p["name"]))); n < 1 || n > 30 {
		return "部门名称长度须在1-30位字符"
	}
	if !phpRequired(p, "status") {
		return "请选择部门状态"
	}
	if !inZeroOne(p["status"]) {
		return "status必须在 0,1 范围内"
	}
	if PHPIsset(p, "sort") && ToInt(p["sort"]) < 0 {
		return "排序值不正确"
	}
	return ""
}

func JobsWriteCheck(p map[string]any, needID bool) string {
	return JobsWriteCheckTaken(p, needID, nil, nil)
}

func JobsWriteCheckTaken(p map[string]any, needID bool, nameTaken, codeTaken func(string) bool) string {
	if needID && !phpRequired(p, "id") {
		return "参数缺失"
	}
	if !phpRequired(p, "name") {
		return "请填写岗位名称"
	}
	if nameTaken != nil && nameTaken(ToString(p["name"])) {
		return "岗位名称已存在"
	}
	if n := len([]rune(ToString(p["name"]))); n < 1 || n > 50 {
		return "岗位名称长度须在1-50位字符"
	}
	if !phpRequired(p, "code") {
		return "请填写岗位编码"
	}
	if codeTaken != nil && codeTaken(ToString(p["code"])) {
		return "岗位编码已存在"
	}
	if !phpRequired(p, "status") {
		return "请选择岗位状态"
	}
	if !inZeroOne(p["status"]) {
		return "岗位状态值错误"
	}
	if PHPIsset(p, "sort") && ToInt(p["sort"]) < 0 {
		return "排序值不正确"
	}
	return ""
}

func PaySceneName(scene int) string {
	switch scene {
	case 1:
		return "H5"
	case 2:
		return "微信公众号"
	case 3:
		return "微信小程序"
	case 4:
		return "APP"
	case 5:
		return "PC"
	default:
		return ""
	}
}

// PayWaySetCheck mirrors PHP PayWayLogic::setPayWay scene rules.
func PayWaySetCheck(p map[string]any) string {
	for key, raw := range p {
		arr, ok := raw.([]any)
		if !ok {
			continue
		}
		name := PaySceneName(ToInt(key))
		defaults, ons := 0, 0
		items := make([]map[string]any, 0, len(arr))
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			items = append(items, m)
			if ToInt(m["is_default"]) == 1 {
				defaults++
			}
			if ToInt(m["status"]) == 1 {
				ons++
			}
		}
		if len(items) == 0 {
			continue
		}
		if defaults == 0 {
			return name + "支付场景缺少默认支付"
		}
		if defaults > 1 {
			return name + "支付场景的默认值只能存在一个"
		}
		if ons == 0 {
			return name + "支付场景至少开启一个支付状态"
		}
		for _, m := range items {
			if ToInt(m["is_default"]) == 1 && ToInt(m["status"]) == 0 {
				return name + "支付场景的默认支付未开启支付状态"
			}
		}
	}
	return ""
}

func UserSetInfoCheck(p map[string]any) string {
	if !phpRequired(p, "field") {
		return "参数缺失"
	}
	if !phpRequired(p, "value") {
		return "值不存在"
	}
	field := ToString(p["field"])
	switch field {
	case "nickname", "account", "sex", "avatar", "real_name":
	default:
		return "参数错误"
	}
	return ""
}

func LoginUpdateUserCheck(p map[string]any) string {
	if !phpRequired(p, "nickname") {
		return "昵称缺少"
	}
	if !phpRequired(p, "avatar") {
		return "头像缺少"
	}
	return ""
}

// WebScanLoginCheck mirrors PHP WebScanLoginValidate field rules (not the cache state check).
func WebScanLoginCheck(p map[string]any) string {
	if !phpRequired(p, "code") {
		return "参数缺失"
	}
	if !phpRequired(p, "state") {
		return "昵称缺少"
	}
	return ""
}

// WechatJsConfigCheck mirrors PHP WechatValidate::sceneJsConfig.
func WechatJsConfigCheck(p map[string]any) string {
	if !phpRequired(p, "url") {
		return "请提供url"
	}
	return ""
}

func OAReplySortCheck(p map[string]any) string {
	if !phpRequired(p, "new_sort") {
		return "请输入新排序值"
	}
	if !isWholeNumber(p["new_sort"]) {
		return "新排序值须为整型"
	}
	if ToInt(p["new_sort"]) < 0 {
		return "新排序值须大于或等于0"
	}
	return ""
}

func PayQueryCheck(p map[string]any) string {
	if !phpRequired(p, "from") {
		return "参数缺失"
	}
	if !phpRequired(p, "order_id") {
		return "订单参数缺失"
	}
	return ""
}

// PayPayCheck mirrors PHP PayValidate scene (from + pay_way + order_id).
func PayPayCheck(p map[string]any) string {
	if !phpRequired(p, "from") {
		return "参数缺失"
	}
	if !phpRequired(p, "pay_way") {
		return "支付方式参数缺失"
	}
	n := ToInt(p["pay_way"])
	if n != 1 && n != 2 && n != 3 {
		return "支付方式参数错误"
	}
	if !phpRequired(p, "order_id") {
		return "订单参数缺失"
	}
	return ""
}

func DbFieldType(typ string) string {
	t := strings.ToLower(strings.TrimSpace(typ))
	switch {
	case strings.HasPrefix(t, "set") || strings.HasPrefix(t, "enum"):
		return "string"
	case strings.Contains(t, "double") || strings.Contains(t, "float") || strings.Contains(t, "decimal") ||
		strings.Contains(t, "real") || strings.Contains(t, "numeric"):
		return "float"
	case strings.Contains(t, "int") || strings.Contains(t, "serial") || strings.Contains(t, "bit"):
		return "int"
	case strings.Contains(t, "bool"):
		return "bool"
	case strings.HasPrefix(t, "timestamp"):
		return "timestamp"
	case strings.HasPrefix(t, "datetime"):
		return "datetime"
	case strings.HasPrefix(t, "date"):
		return "date"
	default:
		return "string"
	}
}

// LoginTerminalCheck mirrors PHP LoginValidate terminal require|in:1,2 default messages.
func LoginTerminalCheck(p map[string]any) string {
	if !phpRequired(p, "terminal") {
		return "terminal不能为空"
	}
	t := ToInt(p["terminal"])
	if t != 1 && t != 2 {
		return "terminal必须在 1,2 范围内"
	}
	return ""
}

// AuthAdminAddCheck mirrors PHP tenant/platform AdminValidate sceneAdd.
func AuthAdminAddCheck(p map[string]any) string {
	return AuthAdminAddCheckTaken(p, nil, nil)
}

func AuthAdminAddCheckTaken(p map[string]any, accountTaken, nameTaken func(string) bool) string {
	if !phpRequired(p, "account") {
		return "账号不能为空"
	}
	if n := len([]rune(ToString(p["account"]))); n < 1 || n > 32 {
		return "账号长度须在1-32位字符"
	}
	if accountTaken != nil && accountTaken(ToString(p["account"])) {
		return "账号已存在"
	}
	if !phpRequired(p, "name") {
		return "名称不能为空"
	}
	if n := len([]rune(ToString(p["name"]))); n < 1 || n > 16 {
		return "名称须在1-16位字符"
	}
	if nameTaken != nil && nameTaken(ToString(p["name"])) {
		return "名称已存在"
	}
	if !phpRequired(p, "password") {
		return "密码不能为空"
	}
	if n := len(ToString(p["password"])); n < 6 || n > 32 {
		return "密码长度须在6-32位字符"
	}
	if !phpRequired(p, "password_confirm") {
		return "确认密码不能为空"
	}
	if ToString(p["password"]) != ToString(p["password_confirm"]) {
		return "两次输入的密码不一致"
	}
	if !phpRequired(p, "role_id") {
		return "请选择角色"
	}
	if !phpRequired(p, "multipoint_login") {
		return "请选择是否支持多处登录"
	}
	mp := ToInt(p["multipoint_login"])
	if mp != 0 && mp != 1 {
		return "多处登录状态值为误"
	}
	return ""
}

// AuthAdminEditCheck mirrors PHP tenant/platform AdminValidate sceneEdit.
// Caller must already reject missing/unknown id (管理员id不能为空 / 管理员不存在).
func AuthAdminEditCheck(p map[string]any, isRoot bool) string {
	return AuthAdminEditCheckTaken(p, isRoot, nil, nil)
}

func AuthAdminEditCheckTaken(p map[string]any, isRoot bool, accountTaken, nameTaken func(string) bool) string {
	if !phpRequired(p, "account") {
		return "账号不能为空"
	}
	if n := len([]rune(ToString(p["account"]))); n < 1 || n > 32 {
		return "账号长度须在1-32位字符"
	}
	if accountTaken != nil && accountTaken(ToString(p["account"])) {
		return "账号已存在"
	}
	if !phpRequired(p, "name") {
		return "名称不能为空"
	}
	if n := len([]rune(ToString(p["name"]))); n < 1 || n > 16 {
		return "名称须在1-16位字符"
	}
	if nameTaken != nil && nameTaken(ToString(p["name"])) {
		return "名称已存在"
	}
	pwd := ToString(p["password"])
	confirm := ToString(p["password_confirm"])
	if pwd != "" || confirm != "" {
		if n := len(pwd); n < 6 || n > 32 {
			return "密码长度须在6-32位字符"
		}
	}
	if phpRequired(p, "password") {
		if !phpRequired(p, "password_confirm") {
			return "确认密码不能为空"
		}
		if pwd != confirm {
			return "两次输入的密码不一致"
		}
	}
	if !isRoot && phpEmptyRole(p) {
		return "请选择角色"
	}
	if !phpRequired(p, "disable") {
		return "请选择状态"
	}
	d := ToInt(p["disable"])
	if d != 0 && d != 1 {
		return "状态值错误"
	}
	if d == 1 && isRoot {
		return "超级管理员不允许被禁用"
	}
	if !phpRequired(p, "multipoint_login") {
		return "请选择是否支持多处登录"
	}
	mp := ToInt(p["multipoint_login"])
	if mp != 0 && mp != 1 {
		return "多处登录状态值为误"
	}
	return ""
}

func phpEmptyRole(p map[string]any) bool {
	v, ok := p["role_id"]
	if !ok || v == nil {
		return true
	}
	switch t := v.(type) {
	case []any:
		return len(t) == 0
	case []string:
		return len(t) == 0
	case []uint:
		return len(t) == 0
	default:
		s := strings.TrimSpace(ToString(v))
		return s == "" || s == "0"
	}
}

func ArticleCateShowCheck(p map[string]any) string {
	if !phpRequired(p, "is_show") {
		return "is_show不能为空"
	}
	v := ToInt(p["is_show"])
	if v != 0 && v != 1 {
		return "is_show必须在 0,1 范围内"
	}
	return ""
}

// UintSlicesChanged is a positional compare (PHP array_diff_assoc plus length).
// Reordering the same IDs counts as a change so tokens expire after role edits.
func UintSlicesChanged(oldIDs, newIDs []uint) bool {
	if len(oldIDs) != len(newIDs) {
		return true
	}
	for i := range oldIDs {
		if oldIDs[i] != newIDs[i] {
			return true
		}
	}
	return false
}

// UpgradeCheck mirrors PHP UpgradeValidate: id + update_type require, update_type must be 1.
func UpgradeCheck(p map[string]any) string {
	if !phpRequired(p, "id") {
		return "参数缺失"
	}
	if !phpRequired(p, "update_type") {
		return "参数缺失"
	}
	if ToInt(p["update_type"]) != 1 {
		return "更新类型错误"
	}
	return ""
}

// CopyrightConfigCheck mirrors PHP WebSettingLogic::setCopyright is_array($params['config']).
func CopyrightConfigCheck(v any) string {
	if v == nil {
		return "参数异常"
	}
	switch v.(type) {
	case []any, []map[string]any, map[string]any:
		return ""
	default:
		return "参数异常"
	}
}

// UpgradeDownloadCheck mirrors PHP downloadPkgValidate require fields.
func UpgradeDownloadCheck(p map[string]any) string {
	if !phpRequired(p, "id") {
		return "参数缺失"
	}
	if !phpRequired(p, "update_type") {
		return "参数缺失"
	}
	return ""
}
