package platformapi

import (
	"os"
	"path/filepath"
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

func TestTableStatusValue(t *testing.T) {
	if tableStatusValue(nil) != nil {
		t.Fatal("nil")
	}
	if tableStatusValue([]byte("la_config")) != "la_config" {
		t.Fatal(tableStatusValue([]byte("la_config")))
	}
	if tableStatusValue([]byte{}) != "" {
		t.Fatal("empty bytes")
	}
}

func TestScanGoModelsNestedPHPPaths(t *testing.T) {
	dir := t.TempDir()
	src := "package model\n\ntype Article struct {}\n\ntype Config struct {}\n"
	if err := os.WriteFile(filepath.Join(dir, "article.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	got := scanGoModels(dir)
	wantArt, wantCfg := false, false
	for _, p := range got {
		if p == `\app\common\model\article\Article` {
			wantArt = true
		}
		if p == `\app\common\model\Config` {
			wantCfg = true
		}
	}
	if !wantArt || !wantCfg {
		t.Fatalf("got %v", got)
	}
}
