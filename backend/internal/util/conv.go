package util

import (
	"encoding/json"
	"strconv"
	"strings"
)

func ToInt(v any) int {
	return int(toInt64(v))
}

func ToFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int:
		return float64(t)
	case int64:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		f, _ := strconv.ParseFloat(ToString(v), 64)
		return f
	}
}

func ToInt64(v any) int64 {
	return toInt64(v)
}

func ToString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case json.Number:
		return t.String()
	default:
		b, _ := json.Marshal(t)
		s := string(b)
		if len(s) >= 2 && s[0] == '"' {
			unq, err := strconv.Unquote(s)
			if err == nil {
				return unq
			}
		}
		return s
	}
}

func ParseInt(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func ContainsInt(list []int, v int) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}

func CamelToPath(s string) string {
	var b strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func ToCamelLower(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i == 0 {
			b.WriteString(strings.ToLower(p))
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(strings.ToLower(p[1:]))
		}
	}
	return strings.ToLower(b.String())
}

func SexDesc(sex int) string {
	switch sex {
	case 1:
		return "男"
	case 2:
		return "女"
	default:
		return "未知"
	}
}

func ChannelDesc(channel int) string {
	switch channel {
	case 1:
		return "微信小程序"
	case 2:
		return "微信公众号"
	case 3:
		return "手机H5"
	case 4:
		return "电脑PC"
	case 5:
		return "苹果APP"
	case 6:
		return "安卓APP"
	default:
		return ""
	}
}

// NoticeTypeDesc mirrors NoticeEnum::getTypeDesc. Unknown types stay empty
// (PHP `$data[$value]` with a missing key).
func NoticeTypeDesc(t int) string {
	switch t {
	case 1:
		return "业务通知"
	case 2:
		return "验证码"
	default:
		return ""
	}
}

// SMSStatusDesc mirrors NoticeSetting::getSmsStatusDescAttr.
func SMSStatusDesc(raw string) string {
	if raw == "" {
		return "停用"
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return "停用"
	}
	if ToInt(m["status"]) == 1 {
		return "启用"
	}
	return "停用"
}

func MoneyString(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// EmptyToNil matches ThinkPHP JSON encoding of empty varchar/decimal-adjacent
// columns that come out as null rather than "".
func EmptyToNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// FormatAmount mirrors PHP format_amount(): strip trailing zeros down to
// integer, one decimal, or the original float.
func FormatAmount(v float64) any {
	iv := int64(v)
	if v == float64(iv) {
		return iv
	}
	one := strconv.FormatFloat(v, 'f', 1, 64)
	if parsed, err := strconv.ParseFloat(one, 64); err == nil && parsed == v {
		return one
	}
	return v
}

func InFold(list []string, v string) bool {
	v = strings.TrimSpace(v)
	for _, item := range list {
		if strings.EqualFold(item, v) {
			return true
		}
	}
	return false
}
