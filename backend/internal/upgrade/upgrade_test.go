package upgrade

import (
	"strings"
	"testing"
)

func TestFormatLists(t *testing.T) {
	rows := []any{
		map[string]any{
			"version_no":       "2.0.0",
			"uniapp_publish":   1,
			"pc_admin_publish": 0,
			"pc_shop_publish":  1,
			"publish_content":  "",
			"update_content": []any{
				map[string]any{"type": 1, "update_function": "A"},
				map[string]any{"type": 2, "update_function": "B"},
				map[string]any{"type": 3, "update_function": "C"},
			},
		},
		map[string]any{
			"version_no":       "1.0.5",
			"uniapp_publish":   0,
			"pc_admin_publish": 0,
			"pc_shop_publish":  0,
			"publish_content":  "x",
			"update_content":   []any{},
		},
	}
	out := FormatLists(rows, 1, "1.0.5")
	if len(out) != 2 {
		t.Fatalf("len=%d", len(out))
	}
	if out[0]["new_version"] != 1 {
		t.Fatalf("new_version=%v", out[0]["new_version"])
	}
	if out[0]["able_update"] != 1 || out[0]["version_str"] != "系统可更新至此版本" {
		t.Fatalf("first=%v", out[0])
	}
	if out[1]["able_update"] != 0 || out[1]["version_str"] != "您的系统当前处于此版本" {
		t.Fatalf("current=%v", out[1])
	}
	notice, _ := out[0]["notice"].([]any)
	if len(notice) != 3 || notice[0] != "更新至当前版本后需重新发布手机端前端前台" || notice[2] != "" {
		t.Fatalf("notice=%v", notice)
	}
	add, _ := out[0]["add"].([]any)
	if len(add) != 1 || add[0] != "新增:A" {
		t.Fatalf("add=%v", add)
	}
	desc, _ := out[0]["content_desc"].([]any)
	if len(desc) != 3 || desc[1] != "优化:B" || desc[2] != "修复:C" {
		t.Fatalf("content_desc=%v", desc)
	}
	if _, ok := out[0]["update_content"]; ok {
		t.Fatal("update_content should be removed")
	}
	page2 := FormatLists([]any{map[string]any{
		"version_no": "2.0.0", "uniapp_publish": 0, "pc_admin_publish": 0, "pc_shop_publish": 0, "publish_content": "",
	}}, 2, "1.0.5")
	if page2[0]["new_version"] != 0 {
		t.Fatalf("page2 new_version=%v", page2[0]["new_version"])
	}
}

func TestCheckAbleUpgrade(t *testing.T) {
	remote := []any{
		map[string]any{"id": 3, "version_no": "1.0.7"},
		map[string]any{"id": 2, "version_no": "1.0.6"},
		map[string]any{"id": 1, "version_no": "1.0.5"},
	}
	payload := map[string]any{"lists": remote}

	if checkAbleUpgrade("1.0.5", nil, payload) != "未获取到对应版本信息" {
		t.Fatal("missing target")
	}
	if !strings.Contains(checkAbleUpgrade("1.0.5", map[string]any{"id": 9, "version_no": "1.0.4"}, payload), "逐个版本") {
		t.Fatal("local newer than target")
	}
	if checkAbleUpgrade("1.0.5", map[string]any{"id": 2, "version_no": "1.0.6"}, map[string]any{}) != "获取更新数据失败" {
		t.Fatal("empty remote")
	}
	if checkAbleUpgrade("1.0.7", map[string]any{"id": 3, "version_no": "1.0.7"}, payload) != "已为最新版本" {
		t.Fatal("already latest")
	}
	if !strings.Contains(checkAbleUpgrade("1.0.5", map[string]any{"id": 3, "version_no": "1.0.7"}, payload), "逐个版本") {
		t.Fatal("skip versions")
	}
	if got := checkAbleUpgrade("1.0.5", map[string]any{"id": 2, "version_no": "1.0.6"}, payload); got != "" {
		t.Fatalf("sequential: %s", got)
	}
	if got := checkAbleUpgrade("0.9.0", map[string]any{"id": 2, "version_no": "1.0.6"}, payload); got != "" {
		t.Fatalf("local not in remote should pass: %s", got)
	}
}

func TestPkgLinkName(t *testing.T) {
	if PkgLinkName(1) != "package_link" || PkgLinkName(4) != "uniapp_package_link" {
		t.Fatal(PkgLinkName(1), PkgLinkName(4))
	}
	if PkgLinkName(9) != "未知类型" {
		t.Fatal(PkgLinkName(9))
	}
}

func TestVersionJSON(t *testing.T) {
	if string(versionJSON("2.1.0")) != `{"version":"2.1.0"}` {
		t.Fatalf("%s", versionJSON("2.1.0"))
	}
}

func TestHasPermission(t *testing.T) {
	if HasPermission(nil) || HasPermission(map[string]any{}) {
		t.Fatal("empty")
	}
	if !HasPermission(map[string]any{"has_permission": true}) || !HasPermission(map[string]any{"has_permission": 1}) {
		t.Fatal("true")
	}
	if HasPermission(map[string]any{"has_permission": false}) || HasPermission(map[string]any{"has_permission": 0}) {
		t.Fatal("false")
	}
}
