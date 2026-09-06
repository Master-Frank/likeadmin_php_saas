package platformapi

import (
	"testing"

	"likeadmin/backend/internal/model"
)

func TestGenerateBundleHasPHPShapes(t *testing.T) {
	files := generateBundle(model.GenerateTable{
		Name: "la_demo", ClassDir: "demo", ModuleName: "admin", TableComment: "演示", Author: "likeadmin",
	}, []model.GenerateColumn{
		{ColumnName: "id", ColumnComment: "主键", IsPk: 1, IsLists: 1, ColumnType: "int"},
		{ColumnName: "name", ColumnComment: "名称", IsLists: 1, ColumnType: "varchar(32)"},
	})
	if len(files) < 8 {
		t.Fatalf("want >=8 files got %d", len(files))
	}
	types := map[string]bool{}
	for _, f := range files {
		types[f["type"].(string)] = true
		if f["content"] == "" || f["name"] == "" {
			t.Fatalf("empty file %+v", f)
		}
	}
	for _, want := range []string{"php", "vue", "sql"} {
		if !types[want] {
			t.Fatalf("missing type %s", want)
		}
	}
}
