package biz

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"likeadmin/backend/internal/util"
)

const (
	NoticeSystem = 1
	NoticeSMS    = 2
	NoticeOA     = 3
	NoticeMNP    = 4
)

var noticeSceneVars = map[int]string{
	101: "验证码:code",
	102: "验证码:code",
	103: "验证码:code",
	104: "验证码:code",
}

var noticeSmsExample = map[int]string{
	101: "您正在登录，验证码${code}，切勿将验证码泄露于他人，本条验证码有效期5分钟。",
	102: "您正在绑定手机号，验证码${code}，切勿将验证码泄露于他人，本条验证码有效期5分钟。",
	103: "您正在变更手机号，验证码${code}，切勿将验证码泄露于他人，本条验证码有效期5分钟。",
	104: "您正在找回登录密码，验证码${code}，切勿将验证码泄露于他人，本条验证码有效期5分钟。",
}

func NoticeTypeDesc(typ int) string {
	switch typ {
	case 1:
		return "业务通知"
	case 2:
		return "验证码"
	default:
		return ""
	}
}

// FormatNoticeDetail mirrors PHP NoticeLogic::detail (platform + tenant).
func FormatNoticeDetail(id uint, typ, sceneID int, sceneName, sceneDesc, system, sms, oa, mnp, support string) any {
	if id == 0 {
		return []any{}
	}
	systemNotice := noticeOrDefault(system, map[string]any{"title": "", "content": "", "status": 0})
	smsNotice := noticeOrDefault(sms, map[string]any{"template_id": "", "content": "", "status": 0})
	oaNotice := noticeOrDefault(oa, map[string]any{
		"template_id": "", "template_sn": "", "name": "", "first": "", "remark": "", "tpl": []any{}, "status": 0,
	})
	mnpNotice := noticeOrDefault(mnp, map[string]any{
		"template_id": "", "template_sn": "", "name": "", "tpl": []any{}, "status": 0,
	})
	systemNotice["tips"] = NoticeOperationTips(NoticeSystem, sceneID)
	smsNotice["tips"] = NoticeOperationTips(NoticeSMS, sceneID)
	// PHP tenant/platform NoticeLogic assigns OA tips with MNP type.
	oaNotice["tips"] = NoticeOperationTips(NoticeMNP, sceneID)
	mnpNotice["tips"] = NoticeOperationTips(NoticeMNP, sceneID)
	systemNotice["is_show"] = supportHas(support, NoticeSystem)
	smsNotice["is_show"] = supportHas(support, NoticeSMS)
	oaNotice["is_show"] = supportHas(support, NoticeOA)
	mnpNotice["is_show"] = supportHas(support, NoticeMNP)
	return map[string]any{
		"id":            id,
		"type":          NoticeTypeDesc(typ),
		"scene_id":      sceneID,
		"scene_name":    sceneName,
		"scene_desc":    sceneDesc,
		"system_notice": systemNotice,
		"sms_notice":    smsNotice,
		"oa_notice":     oaNotice,
		"mnp_notice":    mnpNotice,
		"support":       support,
		"default":       "",
	}
}

func NoticeOperationTips(kind, sceneID int) []string {
	tips := []string{}
	if v, ok := noticeSceneVars[sceneID]; ok {
		tips = append(tips, "可选变量 "+v)
	}
	switch kind {
	case NoticeSystem:
	case NoticeSMS:
		if ex, ok := noticeSmsExample[sceneID]; ok {
			tips = append(tips, "示例："+ex)
		}
		tips = append(tips, "生效条件：1、管理后台完成短信设置。 2、第三方短信平台申请模板。")
	case NoticeOA:
		tips = append(tips, "配置路径：公众号后台 > 广告与服务 > 模板消息")
		tips = append(tips, "推荐行业：主营行业：IT科技/互联网|电子商务 副营行业：消费品/消费品")
	case NoticeMNP:
		tips = append(tips, "配置路径：小程序后台 > 功能 > 订阅消息")
	}
	return tips
}

// ApplyNoticeSet validates and encodes PHP NoticeLogic::set payload.
// template may be a list (PHP) or an object keyed by *_notice (Vue pick()).
func ApplyNoticeSet(exists bool, id uint, template any) (map[string]any, error) {
	if !exists || id == 0 {
		return nil, errors.New("通知配置不存在")
	}
	items, err := ParseNoticeTemplates(template)
	if err != nil {
		return nil, err
	}
	updates := map[string]any{}
	for _, item := range items {
		if err := CheckNoticeItem(item); err != nil {
			return nil, err
		}
		typ := util.ToString(item["type"])
		raw, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		updates[typ+"_notice"] = string(raw)
	}
	return updates, nil
}

func ParseNoticeTemplates(raw any) ([]map[string]any, error) {
	if raw == nil {
		return nil, errors.New("模板配置不存在或格式错误")
	}
	switch t := raw.(type) {
	case []any:
		if len(t) == 0 {
			return nil, errors.New("模板配置不存在或格式错误")
		}
		out := make([]map[string]any, 0, len(t))
		for _, item := range t {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, errors.New("模板项格式错误")
			}
			out = append(out, cloneMap(m))
		}
		return out, nil
	case map[string]any:
		if len(t) == 0 {
			return nil, errors.New("模板配置不存在或格式错误")
		}
		order := []string{"system_notice", "sms_notice", "oa_notice", "mnp_notice"}
		seen := map[string]bool{}
		out := make([]map[string]any, 0, len(t))
		appendItem := func(key string, v any) error {
			m, ok := v.(map[string]any)
			if !ok {
				return errors.New("模板项格式错误")
			}
			item := cloneMap(m)
			if util.ToString(item["type"]) == "" {
				item["type"] = strings.TrimSuffix(key, "_notice")
			}
			out = append(out, item)
			return nil
		}
		for _, key := range order {
			v, ok := t[key]
			if !ok {
				continue
			}
			seen[key] = true
			if err := appendItem(key, v); err != nil {
				return nil, err
			}
		}
		for key, v := range t {
			if seen[key] {
				continue
			}
			if err := appendItem(key, v); err != nil {
				return nil, err
			}
		}
		if len(out) == 0 {
			return nil, errors.New("模板配置不存在或格式错误")
		}
		return out, nil
	default:
		return nil, errors.New("模板配置不存在或格式错误")
	}
}

func CheckNoticeItem(item map[string]any) error {
	if item == nil {
		return errors.New("模板项格式错误")
	}
	typ := util.ToString(item["type"])
	switch typ {
	case "system":
		if !mapHas(item, "title") || !mapHas(item, "content") || !mapHas(item, "status") {
			return errors.New("系统通知必填参数：title、content、status")
		}
	case "sms":
		if !mapHas(item, "template_id") || !mapHas(item, "content") || !mapHas(item, "status") {
			return errors.New("短信通知必填参数：template_id、content、status")
		}
	case "oa":
		need := []string{"template_id", "template_sn", "name", "first", "remark", "tpl", "status"}
		for _, k := range need {
			if !mapHas(item, k) {
				return errors.New("微信模板消息必填参数：template_id、template_sn、name、first、remark、tpl、status")
			}
		}
	case "mnp":
		need := []string{"template_id", "template_sn", "name", "tpl", "status"}
		for _, k := range need {
			if !mapHas(item, k) {
				return errors.New("微信模板消息必填参数：template_id、template_sn、name、tpl、status")
			}
		}
	default:
		return errors.New("模板项缺少模板类型或模板类型有误")
	}
	return nil
}

func noticeOrDefault(raw string, fallback map[string]any) map[string]any {
	if m := decodeNoticeObject(raw); m != nil {
		return m
	}
	return cloneMap(fallback)
}

func decodeNoticeObject(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		if len(t) == 0 {
			return nil
		}
		return t
	default:
		return nil
	}
}

func supportHas(support string, code int) bool {
	want := strconv.Itoa(code)
	for _, part := range strings.Split(support, ",") {
		if strings.TrimSpace(part) == want {
			return true
		}
	}
	return false
}

func mapHas(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

func cloneMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok && m != nil {
		return m
	}
	return map[string]any{}
}
