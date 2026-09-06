package util

import (
	"regexp"
	"strings"
	"unicode"
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
	if n := len(password); n < 6 || n > 20 {
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
	n := len(password)
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
	if strings.TrimSpace(name) == "" {
		return "请填写分组名称"
	}
	if n := len([]rune(name)); n > 20 {
		return "分组名称长度须为20字符内"
	}
	return ""
}

func FileMoveCheck(p map[string]any, ids []uint) string {
	if _, ok := p["ids"]; !ok || len(ids) == 0 {
		return "缺少ids参数"
	}
	if _, ok := p["cid"]; !ok {
		return "缺少cid参数"
	}
	return ""
}

func FileDeleteCheck(p map[string]any, ids []uint) string {
	if _, ok := p["ids"]; !ok || len(ids) == 0 {
		return "缺少ids参数"
	}
	return ""
}

func FileIDCheck(p map[string]any) string {
	if _, ok := p["id"]; !ok || ToInt(p["id"]) == 0 {
		return "缺少id参数"
	}
	return ""
}

func FileAddCateCheck(p map[string]any) string {
	if _, ok := p["type"]; !ok {
		return "缺少type参数"
	}
	typ := ToInt(p["type"])
	if typ != 10 && typ != 20 && typ != 30 {
		return "type必须在 10,20,30 范围内"
	}
	if _, ok := p["pid"]; !ok {
		return "缺少pid参数"
	}
	return FileNameCheck(ToString(p["name"]))
}

func FileEditCateCheck(p map[string]any) string {
	if msg := FileIDCheck(p); msg != "" {
		return msg
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
	var letter, digit bool
	for _, r := range pwd {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			letter = true
		case r >= '0' && r <= '9':
			digit = true
		default:
			return "密码须为字母数字组合"
		}
	}
	if !letter || !digit {
		return "密码须为字母数字组合"
	}
	if _, ok := p["password_confirm"]; !ok || strings.TrimSpace(ToString(p["password_confirm"])) == "" {
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

func OAReplyWriteCheck(p map[string]any, needID bool) string {
	if needID {
		if _, ok := p["id"]; !ok || ToInt(p["id"]) == 0 {
			return "参数缺失"
		}
	}
	if _, ok := p["reply_type"]; !ok {
		return "请输入回复类型"
	}
	rt := ToInt(p["reply_type"])
	if rt != 1 && rt != 2 && rt != 3 {
		return "回复类型状态值错误"
	}
	if strings.TrimSpace(ToString(p["name"])) == "" {
		return "请输入规则名称"
	}
	if _, ok := p["content_type"]; !ok {
		return "请选择内容类型"
	}
	if ToInt(p["content_type"]) != 1 {
		return "内容类型状态值有误"
	}
	if strings.TrimSpace(ToString(p["content"])) == "" {
		return "请输入回复内容"
	}
	if _, ok := p["status"]; !ok {
		return "请选择启用状态"
	}
	st := ToInt(p["status"])
	if st != 0 && st != 1 {
		return "启用状态值错误"
	}
	if rt == 2 {
		if strings.TrimSpace(ToString(p["keyword"])) == "" {
			return "请输入关键词"
		}
		if _, ok := p["matching_type"]; !ok {
			return "请选择匹配类型"
		}
		mt := ToInt(p["matching_type"])
		if mt != 1 && mt != 2 {
			return "匹配类型状态值错误"
		}
		if _, ok := p["sort"]; !ok {
			return "请输入排序值"
		}
		if _, ok := p["reply_num"]; !ok {
			return "请选择回复数量"
		}
		if ToInt(p["reply_num"]) != 1 {
			return "回复数量状态值错误"
		}
	}
	return ""
}

func DictTypeWriteCheck(p map[string]any) string {
	name := strings.TrimSpace(ToString(p["name"]))
	if name == "" {
		return "请填写字典名称"
	}
	if n := len([]rune(name)); n > 255 {
		return "字典名称长度须在1~255位字符"
	}
	if strings.TrimSpace(ToString(p["type"])) == "" {
		return "请填写字典类型"
	}
	if _, ok := p["status"]; !ok {
		return "请选择状态"
	}
	st := ToInt(p["status"])
	if st != 0 && st != 1 {
		return "请选择状态"
	}
	if n := len([]rune(ToString(p["remark"]))); n > 200 {
		return "备注长度不能超过200"
	}
	return ""
}

func DictDataWriteCheck(p map[string]any, needTypeID bool) string {
	name := strings.TrimSpace(ToString(p["name"]))
	if name == "" {
		return "请填写字典数据名称"
	}
	if n := len([]rune(name)); n > 255 {
		return "字典数据名称长度须在1-255位字符"
	}
	if strings.TrimSpace(ToString(p["value"])) == "" {
		return "请填写字典数据值"
	}
	if needTypeID {
		if _, ok := p["type_id"]; !ok || ToInt(p["type_id"]) == 0 {
			return "字典类型缺失"
		}
	}
	if _, ok := p["status"]; !ok {
		return "请选择字典数据状态"
	}
	st := ToInt(p["status"])
	if st != 0 && st != 1 {
		return "字典数据状态参数错误"
	}
	return ""
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
