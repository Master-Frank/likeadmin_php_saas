package biz

import (
	"encoding/json"
	"strconv"
	"strings"

	"likeadmin/backend/internal/util"
)

const (
	PayBalance = 1
	PayWechat  = 2
	PayAlipay  = 3
)

type PayConfigInput struct {
	ID            uint
	Name          string
	Icon          string
	Remark        string
	Sort          any
	SortPresent   bool
	Config        any
	ConfigPresent bool
	PayWay        int
	Exists        bool
	NameTaken     bool
}

func CheckPayConfig(in PayConfigInput) string {
	if in.ID == 0 {
		return "id不能为空"
	}
	if strings.TrimSpace(in.Name) == "" {
		return "支付名称不能为空"
	}
	if in.NameTaken {
		return "支付名称已存在"
	}
	if strings.TrimSpace(in.Icon) == "" {
		return "支付图标不能为空"
	}
	if !in.SortPresent {
		return "排序不能为空"
	}
	if !isNumericValue(in.Sort) {
		return "排序必须是纯数字"
	}
	sortVal := util.ToInt(in.Sort)
	if len(strconv.Itoa(sortVal)) > 5 {
		return "排序最大不能超过五位数"
	}
	if !in.Exists {
		return "支付方式不存在"
	}
	if in.PayWay != PayBalance && !in.ConfigPresent {
		return "支付配置不能为空"
	}
	cfg := asMap(in.Config)
	if in.PayWay == PayWechat {
		if emptyPay(cfg, "interface_version") {
			return "微信支付接口版本不能为空"
		}
		if emptyPay(cfg, "merchant_type") {
			return "商户类型不能为空"
		}
		if emptyPay(cfg, "mch_id") {
			return "微信支付商户号不能为空"
		}
		if emptyPay(cfg, "pay_sign_key") {
			return "商户API密钥不能为空"
		}
		if emptyPay(cfg, "apiclient_cert") {
			return "微信支付证书不能为空"
		}
		if emptyPay(cfg, "apiclient_key") {
			return "微信支付证书密钥不能为空"
		}
	}
	if in.PayWay == PayAlipay {
		if emptyPay(cfg, "mode") {
			return "模式不能为空"
		}
		if emptyPay(cfg, "merchant_type") {
			return "商户类型不能为空"
		}
		if emptyPay(cfg, "app_id") {
			return "应用ID不能为空"
		}
		if emptyPay(cfg, "private_key") {
			return "应用私钥不能为空"
		}
		if util.ToString(cfg["mode"]) == "certificate" {
			if emptyPay(cfg, "public_cert") {
				return "应用公钥证书不能为空"
			}
			if emptyPay(cfg, "ali_public_cert") {
				return "支付宝公钥证书不能为空"
			}
			if emptyPay(cfg, "ali_root_cert") {
				return "支付宝根证书不能为空"
			}
		} else if emptyPay(cfg, "ali_public_key") {
			return "支付宝公钥不能为空"
		}
	}
	return ""
}

func BuildPayConfigJSON(payWay int, raw any) string {
	cfg := asMap(raw)
	switch payWay {
	case PayWechat:
		out := map[string]any{
			"interface_version": cfg["interface_version"],
			"merchant_type":     cfg["merchant_type"],
			"mch_id":            cfg["mch_id"],
			"pay_sign_key":      cfg["pay_sign_key"],
			"apiclient_cert":    cfg["apiclient_cert"],
			"apiclient_key":     cfg["apiclient_key"],
		}
		b, _ := json.Marshal(out)
		return string(b)
	case PayAlipay:
		mode := util.ToString(cfg["mode"])
		aliPub, publicCert, aliPubCert, aliRoot := "", "", "", ""
		if mode == "normal_mode" {
			aliPub = util.ToString(cfg["ali_public_key"])
		}
		if mode == "certificate" {
			publicCert = util.ToString(cfg["public_cert"])
			aliPubCert = util.ToString(cfg["ali_public_cert"])
			aliRoot = util.ToString(cfg["ali_root_cert"])
		}
		out := map[string]any{
			"mode":            cfg["mode"],
			"merchant_type":   cfg["merchant_type"],
			"app_id":          cfg["app_id"],
			"private_key":     cfg["private_key"],
			"ali_public_key":  aliPub,
			"public_cert":     publicCert,
			"ali_public_cert": aliPubCert,
			"ali_root_cert":   aliRoot,
		}
		b, _ := json.Marshal(out)
		return string(b)
	default:
		return ""
	}
}

func DecodePayConfig(raw string) any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return nil
	}
	return v
}

func PayConfigView(id uint, name string, payWay int, icon string, sort int, remark, configJSON, domain string) map[string]any {
	return map[string]any{
		"id":      id,
		"name":    name,
		"pay_way": payWay,
		"icon":    icon,
		"sort":    sort,
		"remark":  remark,
		"config":  DecodePayConfig(configJSON),
		"domain":  domain,
	}
}

func emptyPay(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return true
	}
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		return s == "" || s == "0"
	case bool:
		return !t
	case []any:
		return len(t) == 0
	case map[string]any:
		return len(t) == 0
	default:
		return util.ToFloat(v) == 0
	}
}

func isNumericValue(v any) bool {
	switch t := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return false
		}
		_, err := strconv.Atoi(s)
		return err == nil
	default:
		return false
	}
}
