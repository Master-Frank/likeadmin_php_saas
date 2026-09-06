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

func FileRenameCheck(p map[string]any) string {
	return FileEditCateCheck(p)
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
