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
