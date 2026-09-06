package export

import "testing"

func TestLookupLogFields(t *testing.T) {
	spec := Lookup("setting.system.log", "lists")
	if spec.FileName != "系统日志" || len(spec.Fields) != 9 {
		t.Fatalf("%+v", spec)
	}
	if spec.Fields[0].Title != "记录ID" || spec.Fields[1].Key != "action" {
		t.Fatalf("fields %+v", spec.Fields)
	}
}

func TestToRecordsUsesChineseHeaders(t *testing.T) {
	rows := []map[string]any{
		{"id": 1, "action": " 查看系统日志列表", "extra": "drop"},
	}
	rec := toRecords(rows, []Field{{Key: "id", Title: "记录ID"}, {Key: "action", Title: "操作"}})
	if len(rec) != 2 || rec[0][0] != "记录ID" || rec[0][1] != "操作" {
		t.Fatalf("%v", rec)
	}
	if rec[1][0] != "1" || rec[1][1] != " 查看系统日志列表" {
		t.Fatalf("row %v", rec[1])
	}
}

func TestLookupCompactName(t *testing.T) {
	spec := Lookup("tenant.tenantadmin", "lists")
	if spec.FileName != "租户用户列表" {
		t.Fatalf("%+v", spec)
	}
}

func TestFormatCellEnums(t *testing.T) {
	if formatCell("channel", 1) != "微信小程序" {
		t.Fatalf("channel %s", formatCell("channel", 1))
	}
	if formatCell("disable", 0) != "正常" || formatCell("disable", 1) != "禁用" {
		t.Fatalf("disable %s %s", formatCell("disable", 0), formatCell("disable", 1))
	}
	if formatCell("pay_status_text", 1) != "已支付" {
		t.Fatalf("pay %s", formatCell("pay_status_text", 1))
	}
	if formatCell("pay_status_text", "已支付") != "已支付" {
		t.Fatalf("already text %s", formatCell("pay_status_text", "已支付"))
	}
	rec := toRecords([]map[string]any{{"channel": 2, "disable": 1}}, []Field{{Key: "channel", Title: "注册来源"}, {Key: "disable", Title: "是否禁用"}})
	if len(rec) != 2 || rec[1][0] != "微信公众号" || rec[1][1] != "禁用" {
		t.Fatalf("%v", rec)
	}
}
