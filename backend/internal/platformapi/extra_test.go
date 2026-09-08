package platformapi

import "testing"

func TestCrontabWriteCheck(t *testing.T) {
	if crontabWriteCheck(map[string]any{}, false) != "请输入定时任务名称" {
		t.Fatal(crontabWriteCheck(map[string]any{}, false))
	}
	base := map[string]any{"name": "job", "type": nil, "command": "x", "status": 1, "expression": "* * * * *"}
	if crontabWriteCheck(base, false) != "请选择类型" {
		t.Fatal(crontabWriteCheck(base, false))
	}
	base["type"] = 1
	base["status"] = nil
	if crontabWriteCheck(base, false) != "请选择状态" {
		t.Fatal(crontabWriteCheck(base, false))
	}
	base["status"] = 1
	base["expression"] = nil
	if crontabWriteCheck(base, false) != "请输入运行规则" {
		t.Fatal(crontabWriteCheck(base, false))
	}
	base["expression"] = "* * * * *"
	if crontabWriteCheck(base, true) != "参数缺失" {
		t.Fatal(crontabWriteCheck(base, true))
	}
	base["id"] = 1
	if crontabWriteCheck(base, true) != "" {
		t.Fatal(crontabWriteCheck(base, true))
	}
	base["name"] = "   "
	if crontabWriteCheck(base, false) != "" {
		t.Fatal("ThinkPHP require accepts whitespace name")
	}
}
