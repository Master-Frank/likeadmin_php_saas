package generator

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"likeadmin/backend/internal/model"
)

func sampleTable() (model.GenerateTable, []model.GenerateColumn) {
	t := model.GenerateTable{
		Name: "la_config", TableComment: "配置", ClassComment: "配置",
		Author: "likeadmin", ModuleName: "platform", ClassDir: "",
		TemplateType: 0, GenerateType: 0,
		Menu:      `{"pid":0,"type":0,"name":"配置"}`,
		Delete:    `{"type":0,"name":"delete_time"}`,
		Tree:      `{}`,
		Relations: `[]`,
	}
	cols := []model.GenerateColumn{
		{ColumnName: "id", ColumnComment: "主键", ColumnType: "int", IsPk: 1, IsLists: 1},
		{ColumnName: "name", ColumnComment: "名称", ColumnType: "string", IsRequired: 1, IsInsert: 1, IsUpdate: 1, IsLists: 1, IsQuery: 1, QueryType: "like", ViewType: "input"},
		{ColumnName: "status", ColumnComment: "状态", ColumnType: "int", IsInsert: 1, IsUpdate: 1, IsLists: 1, IsQuery: 1, QueryType: "=", ViewType: "select", DictType: "show_status"},
	}
	return t, cols
}

func TestPreviewMatchesPHPFileInfo(t *testing.T) {
	if !stubExists("php/controller") {
		t.Fatalf("php stubs not found under %s", StubDir())
	}
	tbl, cols := sampleTable()
	files := BuildAt(tbl, cols, time.Date(2026, 9, 6, 20, 0, 0, 0, time.Local))
	want := []struct{ name, typ, zip string }{
		{"ConfigController.php", "php", "generate/php/app/platform/controller/ConfigController.php"},
		{"ConfigLists.php", "php", "generate/php/app/platform/lists/ConfigLists.php"},
		{"Config.php", "php", "generate/php/app/common/model/Config.php"},
		{"ConfigValidate.php", "php", "generate/php/app/platform/validate/ConfigValidate.php"},
		{"ConfigLogic.php", "php", "generate/php/app/platform/logic/ConfigLogic.php"},
		{"config.ts", "ts", "generate/vue/src/api/config.ts"},
		{"index.vue", "vue", "generate/vue/src/views/config/index.vue"},
		{"edit.vue", "vue", "generate/vue/src/views/config/edit.vue"},
		{"menu.sql", "sql", "generate/sql/menu.sql"},
	}
	if len(files) != len(want) {
		t.Fatalf("file count %d want %d", len(files), len(want))
	}
	for i, w := range want {
		if files[i].Name != w.name || files[i].Type != w.typ {
			t.Fatalf("file %d name/type = %s/%s want %s/%s", i, files[i].Name, files[i].Type, w.name, w.typ)
		}
		if files[i].ZipName() != w.zip {
			t.Fatalf("file %d zip = %s want %s", i, files[i].ZipName(), w.zip)
		}
		if files[i].Content == "" {
			t.Fatalf("file %d empty content", i)
		}
	}
	ctrl := files[0].Content
	if !strings.Contains(ctrl, "namespace app\\platform\\controller;") {
		t.Fatalf("controller namespace: %s", ctrl[:200])
	}
	if !strings.Contains(ctrl, "class ConfigController extends BaseLikeAdminController") {
		t.Fatalf("controller should match PHP extends quirk")
	}
	if !strings.Contains(files[1].Content, "'%like%' => ['name']") {
		t.Fatalf("lists query missing like: %s", files[1].Content)
	}
	if strings.Contains(files[2].Content, "SoftDelete") {
		t.Fatalf("true-delete model should not use SoftDelete")
	}
	if !strings.Contains(files[5].Content, "url: '/config/lists'") {
		t.Fatalf("vue api route: %s", files[5].Content)
	}
	if !strings.Contains(files[8].Content, "`la_system_menu`") && !strings.Contains(files[8].Content, "`la_system_menu`") {
		if !strings.Contains(files[8].Content, "la_system_menu") {
			t.Fatalf("menu sql table: %s", files[8].Content)
		}
	}
}

func TestPreviewWithClassDirAndSoftDelete(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.ClassDir = "setting"
	tbl.Delete = `{"type":1,"name":"delete_time"}`
	files := Build(tbl, cols)
	if files[0].ZipName() != "generate/php/app/platform/controller/setting/ConfigController.php" {
		t.Fatalf("class_dir zip %s", files[0].ZipName())
	}
	if !strings.Contains(files[0].Content, "namespace app\\platform\\controller\\setting;") {
		t.Fatalf("controller ns with class_dir")
	}
	if !strings.Contains(files[2].Content, "use SoftDelete;") {
		t.Fatalf("soft delete missing")
	}
	if !strings.Contains(files[5].Content, "url: '/setting.config/lists'") {
		t.Fatalf("vue route with class_dir: %s", files[5].Content)
	}
}

func TestPreviewClassDirPreservesCaseInSQL(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.ClassDir = "Setting"
	files := Build(tbl, cols)
	if !strings.Contains(files[8].Content, "Setting.config") {
		t.Fatalf("menu.sql should keep classDir case: %s", files[8].Content)
	}
	if strings.Contains(files[8].Content, "setting.config") {
		t.Fatal("menu.sql must not lowercase classDir")
	}
	if !strings.Contains(files[5].Content, "url: '/setting.config/lists'") {
		t.Fatalf("vue api route is lowercased: %s", files[5].Content)
	}
}

func TestTreePreview(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.TemplateType = 1
	tbl.Tree = `{"tree_id":"id","tree_pid":"pid","tree_name":"name"}`
	cols = append(cols, model.GenerateColumn{ColumnName: "pid", ColumnComment: "父级", ColumnType: "int", IsInsert: 1, IsUpdate: 1, IsLists: 1, ViewType: "input"})
	files := Build(tbl, cols)
	if !strings.Contains(files[1].Content, "linear_to_tree") {
		t.Fatalf("tree lists should use linear_to_tree")
	}
	if !strings.Contains(files[6].Content, `row-key="id"`) {
		t.Fatalf("tree index missing row-key")
	}
	if !strings.Contains(files[7].Content, "el-tree-select") {
		t.Fatalf("tree edit missing treeSelect: %s", files[7].Content)
	}
	if !strings.Contains(files[7].Content, "const treeList = ref") {
		t.Fatalf("tree edit missing treeList const: %s", files[7].Content)
	}
	if !strings.Contains(files[7].Content, "顶级") {
		t.Fatalf("tree edit missing getLists stub: %s", files[7].Content)
	}
}

func TestPreviewRelationsCheckboxAndBetweenTime(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.Relations = `[{"name":"items","model":"PairItem","type":"has_many","local_key":"id","foreign_key":"pid"}]`
	cols = append(cols,
		model.GenerateColumn{ColumnName: "tags", ColumnComment: "标签", ColumnType: "string", IsInsert: 1, IsUpdate: 1, IsLists: 1, ViewType: "checkbox"},
		model.GenerateColumn{ColumnName: "create_time", ColumnComment: "创建时间", ColumnType: "int", IsLists: 1, IsQuery: 1, QueryType: "between", ViewType: "datetime"},
	)
	files := Build(tbl, cols)
	var modelPHP, listsPHP, editVue string
	for _, f := range files {
		switch f.Name {
		case "Config.php":
			modelPHP = f.Content
		case "ConfigLists.php":
			listsPHP = f.Content
		case "edit.vue":
			editVue = f.Content
		}
	}
	if !strings.Contains(modelPHP, "hasMany") || !strings.Contains(modelPHP, "PairItem::class") || !strings.Contains(modelPHP, "function items") {
		t.Fatalf("has_many stub missing: %s", modelPHP)
	}
	if !strings.Contains(listsPHP, "'between_time' => ['create_time']") {
		t.Fatalf("between_time missing: %s", listsPHP)
	}
	if !strings.Contains(editVue, `split(",")`) || !strings.Contains(editVue, "formData.tags") {
		t.Fatalf("checkbox split missing: %s", editVue)
	}
}

func TestPreviewEmptyRelationTypeSkipped(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.Relations = `[{"name":"owner","model":"User","local_key":"id","foreign_key":"user_id"}]`
	files := Build(tbl, cols)
	if strings.Contains(files[2].Content, "hasOne") || strings.Contains(files[2].Content, "function owner") {
		t.Fatalf("empty type must skip relation stub: %s", files[2].Content)
	}
}

func TestPreviewExplicitHasOne(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.Relations = `[{"name":"owner","model":"User","type":"has_one","local_key":"id","foreign_key":"user_id"}]`
	files := Build(tbl, cols)
	if !strings.Contains(files[2].Content, "hasOne") || !strings.Contains(files[2].Content, "User::class") || !strings.Contains(files[2].Content, "function owner") {
		t.Fatalf("has_one stub missing: %s", files[2].Content)
	}
}

func TestClearRuntimeKeepsCurdZip(t *testing.T) {
	root := RuntimeDir()
	if err := os.MkdirAll(filepath.Join(root, "generate", "php"), 0755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(root, "curd-keep.zip")
	if err := os.WriteFile(zipPath, []byte("PK\x03\x04"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "generate", "php", "x.php"), []byte("<?php"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ClearRuntime(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("curd zip should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "generate", "php", "x.php")); err == nil {
		t.Fatal("generated sources should be cleared")
	}
}

func TestZipRuntimeUsesGeneratePrefix(t *testing.T) {
	root := RuntimeDir()
	if err := os.MkdirAll(filepath.Join(root, "php"), 0755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "php", "ZipProbe.php")
	if err := os.WriteFile(src, []byte("<?php"), 0644); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "curd-probe.zip")
	if err := ZipRuntime(zipPath); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	found := false
	for _, f := range r.File {
		if f.Name == "generate/php/ZipProbe.php" {
			found = true
			break
		}
		if strings.HasPrefix(f.Name, "curd-") {
			t.Fatalf("zip should skip curd packages, got %s", f.Name)
		}
	}
	if !found {
		names := make([]string, 0, len(r.File))
		for _, f := range r.File {
			names = append(names, f.Name)
		}
		t.Fatalf("missing generate/php/ZipProbe.php in %v", names)
	}
}

func TestZipRuntimeContentBytes(t *testing.T) {
	root := RuntimeDir()
	if err := os.MkdirAll(filepath.Join(root, "php"), 0755); err != nil {
		t.Fatal(err)
	}
	want := "<?php\n// zip-content-probe\n"
	src := filepath.Join(root, "php", "ZipContent.php")
	if err := os.WriteFile(src, []byte(want), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(src) })
	zipPath := filepath.Join(t.TempDir(), "curd-content.zip")
	if err := ZipRuntime(zipPath); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var got string
	for _, f := range r.File {
		if f.Name != "generate/php/ZipContent.php" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got = string(b)
	}
	if got != want {
		t.Fatalf("zip bytes=%q want=%q", got, want)
	}
}

func TestNaming(t *testing.T) {
	if Studly("user_account") != "UserAccount" {
		t.Fatalf("studly %s", Studly("user_account"))
	}
	if Camel("user_account") != "userAccount" {
		t.Fatalf("camel %s", Camel("user_account"))
	}
	if NoPrefix("la_config") != "config" {
		t.Fatalf("prefix %s", NoPrefix("la_config"))
	}
}

func TestWriteModuleWritesPHP(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.GenerateType = 1
	files := Build(tbl, cols)
	c := newCtx(tbl, cols, time.Now())
	php, vue := 0, 0
	for _, f := range files {
		dest := moduleDest(c, "/tmp/admin", f)
		if strings.HasSuffix(f.Name, ".php") {
			php++
			if dest == "" {
				t.Fatalf("php should be written: %s", f.Name)
			}
			inModule := strings.Contains(dest, filepath.Join("app", c.module))
			inModel := strings.Contains(dest, filepath.Join("app", "common", "model"))
			if !inModule && !inModel {
				t.Fatalf("php dest unexpected: %s -> %s", f.Name, dest)
			}
		}
		if strings.HasSuffix(f.Name, ".ts") || strings.HasSuffix(f.Name, ".vue") || f.Name == "menu.sql" {
			vue++
			if dest == "" {
				t.Fatalf("vue/sql should be written: %s", f.Name)
			}
		}
	}
	if php < 5 || vue < 3 {
		t.Fatalf("php=%d vue=%d", php, vue)
	}
	goFiles := BuildGo(tbl, cols)
	if len(goFiles) != 1 || goFiles[0].Type != "go" || !strings.Contains(goFiles[0].Content, "package generated") {
		t.Fatalf("go files %+v", goFiles)
	}
	if !strings.Contains(goFiles[0].Content, "gencrud") {
		t.Fatalf("go metadata should mention gencrud: %s", goFiles[0].Content)
	}
}

func TestApplyMenuSQLEmpty(t *testing.T) {
	if err := ApplyMenuSQL(nil, "INSERT INTO x"); err != nil {
		t.Fatal(err)
	}
	if err := ApplyMenuSQL(nil, ""); err != nil {
		t.Fatal(err)
	}
}
