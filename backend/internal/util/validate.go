package util

import (
	"regexp"
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
