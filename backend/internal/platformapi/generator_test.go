package platformapi

import (
	"testing"

	"likeadmin/backend/internal/generator"
	"likeadmin/backend/internal/model"
)

func TestGenerateBundleHasPHPShapes(t *testing.T) {
	files := generator.Build(model.GenerateTable{
		Name: "la_demo", ModuleName: "platform", TableComment: "演示", Author: "likeadmin",
	}, []model.GenerateColumn{
		{ColumnName: "id", ColumnComment: "主键", IsPk: 1, IsLists: 1, ColumnType: "int"},
		{ColumnName: "name", ColumnComment: "名称", IsLists: 1, IsInsert: 1, IsUpdate: 1, ColumnType: "string", ViewType: "input"},
	})
	if len(files) != 9 {
		t.Fatalf("want 9 files got %d", len(files))
	}
	types := map[string]bool{}
	for _, f := range files {
		types[f.Type] = true
		if f.Content == "" || f.Name == "" {
			t.Fatalf("empty file %+v", f)
		}
	}
	for _, want := range []string{"php", "ts", "vue", "sql"} {
		if !types[want] {
			t.Fatalf("missing type %s", want)
		}
	}
	if files[0].Name != "DemoController.php" || files[5].Type != "ts" {
		t.Fatalf("php preview names: %s %s", files[0].Name, files[5].Name)
	}
}
