package wechat

import (
	"encoding/json"
	"testing"
)

func TestBuildMenuButtons(t *testing.T) {
	in := []any{
		map[string]any{
			"name":     "首页",
			"type":     "view",
			"url":      "https://example.com",
			"has_menu": false,
			"extra":    "drop-me",
		},
		map[string]any{
			"name":     "更多",
			"has_menu": true,
			"sub_button": []any{
				map[string]any{"name": "点击", "type": "click", "key": "K1", "has_menu": 0},
			},
		},
	}
	got := BuildMenuButtons(in)
	raw, _ := json.Marshal(got)
	var out []map[string]any
	if json.Unmarshal(raw, &out) != nil || len(out) != 2 {
		t.Fatalf("%s", raw)
	}
	if _, ok := out[0]["has_menu"]; ok {
		t.Fatalf("has_menu leaked %s", raw)
	}
	if _, ok := out[0]["extra"]; ok {
		t.Fatalf("ui key leaked %s", raw)
	}
	if out[0]["type"] != "view" || out[0]["url"] != "https://example.com" {
		t.Fatalf("leaf %+v", out[0])
	}
	sub, _ := out[1]["sub_button"].([]any)
	if out[1]["name"] != "更多" || out[1]["type"] != nil || len(sub) != 1 {
		t.Fatalf("parent %+v", out[1])
	}
}
