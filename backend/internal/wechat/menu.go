package wechat

import (
	"strings"

	"likeadmin/backend/internal/util"
)

var wechatMenuFields = []string{"type", "name", "key", "url", "appid", "pagepath", "media_id"}

// BuildMenuButtons strips admin UI keys (has_menu, etc.) so WeChat menu/create
// gets the official button shape.
func BuildMenuButtons(menu []any) []any {
	out := make([]any, 0, len(menu))
	for _, item := range menu {
		m, ok := item.(map[string]any)
		if !ok || m == nil {
			continue
		}
		out = append(out, buildMenuButton(m))
	}
	return out
}

func buildMenuButton(m map[string]any) map[string]any {
	name := strings.TrimSpace(util.ToString(m["name"]))
	sub, _ := m["sub_button"].([]any)
	if len(sub) > 0 {
		return map[string]any{
			"name":       name,
			"sub_button": BuildMenuButtons(sub),
		}
	}
	btn := map[string]any{}
	if name != "" {
		btn["name"] = name
	}
	for _, k := range wechatMenuFields {
		if k == "name" {
			continue
		}
		if v, ok := m[k]; ok && v != nil && util.ToString(v) != "" {
			btn[k] = v
		}
	}
	return btn
}
