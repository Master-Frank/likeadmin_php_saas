package generator

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"

	"gorm.io/gorm"
)

func initGeneratorDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		return true
	}
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if err := bootstrap.Init(cfg); err != nil {
		t.Log(err)
		return false
	}
	return bootstrap.DB != nil
}

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

func TestPreviewEmptyRelationKeysStayEmpty(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.Relations = `[{"name":"owner","model":"User","type":"has_one","local_key":"","foreign_key":""}]`
	files := Build(tbl, cols)
	if !strings.Contains(files[2].Content, "hasOne") {
		t.Fatalf("has_one stub missing: %s", files[2].Content)
	}
	if strings.Contains(files[2].Content, "hasOne(User::class, 'id', 'id')") {
		t.Fatalf("empty keys must not default to id: %s", files[2].Content)
	}
	if !strings.Contains(files[2].Content, "hasOne(User::class, '', '')") && !strings.Contains(files[2].Content, "hasOne(User::class, \"\", \"\")") {
		t.Fatalf("PHP ModelGenerator writes empty keys as-is: %s", files[2].Content)
	}
}

func TestClearRuntimeRemovesCurdZip(t *testing.T) {
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
	if _, err := os.Stat(zipPath); err == nil {
		t.Fatal("PHP delGenerateDirContent removes prior curd-*.zip")
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

func TestApplyMenuSQLLastInsertIDOnRealDB(t *testing.T) {
	if !initGeneratorDB(t) {
		t.Skip("no database")
	}
	tbl, cols := sampleTable()
	tbl.Menu = `{"pid":0,"type":1,"name":"GoMenu990014"}`
	files := BuildAt(tbl, cols, time.Date(2026, 9, 8, 12, 0, 0, 0, time.Local))
	var sqlText string
	for _, f := range files {
		if f.Name == "menu.sql" {
			sqlText = f.Content
			break
		}
	}
	if sqlText == "" || !strings.Contains(sqlText, "LAST_INSERT_ID") || !strings.Contains(sqlText, "@pid") {
		t.Fatalf("menu.sql missing LAST_INSERT_ID/@pid: %s", sqlText)
	}
	t.Cleanup(func() {
		var rows []model.SystemMenu
		bootstrap.DB.Where("name = ? AND type = ? AND perms = ?", "GoMenu990014", "C", "config/lists").Find(&rows)
		for _, p := range rows {
			_ = bootstrap.DB.Where("pid = ?", p.ID).Delete(&model.SystemMenu{}).Error
			_ = bootstrap.DB.Where("id = ?", p.ID).Delete(&model.SystemMenu{}).Error
		}
	})
	if err := ApplyMenuSQL(bootstrap.DB, sqlText); err != nil {
		t.Fatal(err)
	}
	var parent model.SystemMenu
	if err := bootstrap.DB.Where("name = ? AND type = ? AND perms = ?", "GoMenu990014", "C", "config/lists").
		Order("id desc").First(&parent).Error; err != nil {
		t.Fatalf("parent menu: %v", err)
	}
	if parent.Pid != 0 {
		t.Fatalf("parent pid=%d", parent.Pid)
	}
	var kids []model.SystemMenu
	bootstrap.DB.Where("pid = ?", parent.ID).Order("id asc").Find(&kids)
	if len(kids) != 3 {
		t.Fatalf("children=%d want 3 (LAST_INSERT_ID/@pid)", len(kids))
	}
	want := []string{"添加", "编辑", "删除"}
	for i, name := range want {
		if kids[i].Name != name || kids[i].Type != "A" {
			t.Fatalf("child %d %+v want %s", i, kids[i], name)
		}
	}
}

func TestZipRuntimeFileByFileFromBuild(t *testing.T) {
	if !stubExists("php/controller") {
		t.Fatalf("php stubs not found under %s", StubDir())
	}
	tbl, cols := sampleTable()
	files := BuildAt(tbl, cols, time.Date(2026, 9, 6, 20, 0, 0, 0, time.Local))
	root := RuntimeDir()
	written := make([]string, 0, len(files))
	t.Cleanup(func() {
		for _, p := range written {
			_ = os.Remove(p)
		}
	})
	if err := WriteRuntime(files); err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		written = append(written, filepath.Join(root, filepath.FromSlash(f.RelPath)))
	}
	zipPath := filepath.Join(t.TempDir(), "curd-build.zip")
	if err := ZipRuntime(zipPath); err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got := map[string]string{}
	for _, zf := range r.File {
		rc, err := zf.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		got[zf.Name] = string(b)
	}
	for _, f := range files {
		body, ok := got[f.ZipName()]
		if !ok {
			t.Fatalf("zip missing %s (have %d entries)", f.ZipName(), len(got))
		}
		if body != f.Content {
			t.Fatalf("%s zip bytes mismatch want %d got %d", f.ZipName(), len(f.Content), len(body))
		}
	}
}

func TestModuleDestsWritesRealFrontend(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.ModuleName = "platform"
	c := newCtx(tbl, cols, time.Now())
	var ts File
	for _, f := range Build(tbl, cols) {
		if strings.HasSuffix(f.Name, ".ts") {
			ts = f
			break
		}
	}
	if ts.Name == "" {
		t.Fatal("missing vue api")
	}
	dests := moduleDests(c, "/tmp/admin", ts)
	if len(dests) < 2 {
		t.Fatalf("dests=%v", dests)
	}
	if !strings.HasPrefix(dests[0], "/tmp/admin") {
		t.Fatalf("admin dest %s", dests[0])
	}
	plat := filepath.Join(RepoRoot(), "platform", "src", "api", ts.Name)
	if !containsPath(dests, plat) {
		t.Fatalf("missing platform dest %s in %v", plat, dests)
	}

	tbl.ModuleName = "tenant"
	c = newCtx(tbl, cols, time.Now())
	dests = moduleDests(c, "/tmp/admin", ts)
	ten := filepath.Join(RepoRoot(), "tenant", "src", "api", ts.Name)
	if !containsPath(dests, ten) {
		t.Fatalf("missing tenant dest %s in %v", ten, dests)
	}

	phpDests := moduleDests(c, "/tmp/admin", File{Name: "ConfigController.php"})
	if len(phpDests) != 1 {
		t.Fatalf("php should stay a single dest: %v", phpDests)
	}
}

func TestWriteModuleWritesFrontendFiles(t *testing.T) {
	tbl, cols := sampleTable()
	tbl.Name = "la_go_genvue_990016"
	tbl.ModuleName = "tenant"
	tbl.GenerateType = 1
	files := Build(tbl, cols)
	c := newCtx(tbl, nil, time.Now())
	admin := filepath.Join(RepoRoot(), "admin", "src")
	var written []string
	for _, f := range files {
		written = append(written, moduleDests(c, admin, f)...)
	}
	for _, f := range BuildGo(tbl, nil) {
		written = append(written, filepath.Join(RepoRoot(), "backend", "internal", "generated", f.Name))
	}
	t.Cleanup(func() {
		for _, p := range written {
			_ = os.Remove(p)
		}
	})
	if err := WriteModule(tbl, files); err != nil {
		t.Fatal(err)
	}
	tenTS := filepath.Join(RepoRoot(), "tenant", "src", "api", "go_genvue_990016.ts")
	if _, err := os.Stat(tenTS); err != nil {
		t.Fatalf("tenant vue api not written: %v", err)
	}
	adminTS := filepath.Join(RepoRoot(), "admin", "src", "api", "go_genvue_990016.ts")
	if _, err := os.Stat(adminTS); err != nil {
		t.Fatalf("admin vue api not written: %v", err)
	}
	tenIndex := filepath.Join(RepoRoot(), "tenant", "src", "views", "go_genvue_990016", "index.vue")
	if _, err := os.Stat(tenIndex); err != nil {
		t.Fatalf("tenant index.vue not written: %v", err)
	}
}

func TestIsTenantModule(t *testing.T) {
	if !IsTenantModule(model.GenerateTable{ModuleName: "tenant"}) {
		t.Fatal("tenant")
	}
	if !IsTenantModule(model.GenerateTable{ModuleName: "tenantapi"}) {
		t.Fatal("tenantapi")
	}
	if IsTenantModule(model.GenerateTable{ModuleName: "platform"}) {
		t.Fatal("platform is not tenant")
	}
}

func TestRewriteMenuSQLForTenant(t *testing.T) {
	src := "INSERT INTO `la_system_menu`(`pid`, `type`, `name`)\n VALUES (0, 'C', 'X');\nSELECT @pid := LAST_INSERT_ID();\nINSERT INTO `la_system_menu`(`pid`, `type`, `name`)\n VALUES (@pid, 'A', '添加');"
	got := RewriteMenuSQLForTenant(src, "la_tenant_system_menu_t990017", 990017)
	if !strings.Contains(got, "`la_tenant_system_menu_t990017`") {
		t.Fatalf("table: %s", got)
	}
	if strings.Contains(got, "la_system_menu") {
		t.Fatalf("still platform table: %s", got)
	}
	if !strings.Contains(got, "(`tenant_id`, `pid`,") {
		t.Fatalf("missing tenant_id column: %s", got)
	}
	if !strings.Contains(got, "VALUES (990017, 0, 'C', 'X')") {
		t.Fatalf("parent values: %s", got)
	}
	if !strings.Contains(got, "VALUES (990017, @pid, 'A', '添加')") {
		t.Fatalf("child values: %s", got)
	}
	if !strings.Contains(got, "SELECT @pid := LAST_INSERT_ID()") {
		t.Fatal("must keep LAST_INSERT_ID")
	}
	if RewriteMenuSQLForTenant("", "la_tenant_system_menu", 0) != "" {
		t.Fatal("empty sql")
	}
}

func TestApplyTenantMenusLastInsertIDOnRealDB(t *testing.T) {
	if !initGeneratorDB(t) {
		t.Skip("no database")
	}
	const sharedID uint = 990016
	const shardID uint = 990017
	const shardSN = "t990017"
	const menuName = "GoTenantMenu990016"
	db := bootstrap.DB
	shardTable := "la_tenant_system_menu_" + shardSN
	cleanup := func() {
		cleanupTenantMenusByName(db, "la_tenant_system_menu", menuName)
		cleanupTenantMenusByName(db, shardTable, menuName)
		_ = db.Exec("DROP TABLE IF EXISTS " + shardTable).Error
		_ = db.Where("id IN ?", []uint{sharedID, shardID}).Delete(&model.Tenant{}).Error
	}
	cleanup()
	t.Cleanup(cleanup)

	now := time.Now().Unix()
	if err := db.Create(&model.Tenant{ID: sharedID, SN: "t990016", Name: "gen-menu-shared", CreateTime: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE " + shardTable + " LIKE la_tenant_system_menu").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Tenant{ID: shardID, SN: shardSN, Name: "gen-menu-shard", Tactics: 1, CreateTime: now}).Error; err != nil {
		t.Fatal(err)
	}

	tbl, cols := sampleTable()
	tbl.ModuleName = "tenant"
	tbl.Menu = `{"pid":0,"type":1,"name":"` + menuName + `"}`
	files := BuildAt(tbl, cols, time.Date(2026, 9, 8, 13, 0, 0, 0, time.Local))
	var sqlText string
	for _, f := range files {
		if f.Name == "menu.sql" {
			sqlText = f.Content
			break
		}
	}
	if sqlText == "" || !strings.Contains(sqlText, "la_system_menu") {
		t.Fatalf("menu.sql: %s", sqlText)
	}
	if err := applyTenantMenusTo(db, sqlText, []model.Tenant{
		{ID: sharedID, SN: "t990016", Tactics: 0},
		{ID: shardID, SN: shardSN, Tactics: 1},
	}); err != nil {
		t.Fatal(err)
	}

	assertTenantMenuTree(t, db, "la_tenant_system_menu", menuName, 0)
	assertTenantMenuTree(t, db, "la_tenant_system_menu", menuName, sharedID)
	assertTenantMenuTree(t, db, shardTable, menuName, shardID)
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func cleanupTenantMenusByName(db *gorm.DB, table, name string) {
	var rows []model.TenantSystemMenu
	db.Table(table).Where("name = ? AND type = ?", name, "C").Find(&rows)
	for _, p := range rows {
		_ = db.Exec("DELETE FROM `"+table+"` WHERE pid = ?", p.ID).Error
		_ = db.Exec("DELETE FROM `"+table+"` WHERE id = ?", p.ID).Error
	}
}

func assertTenantMenuTree(t *testing.T, db *gorm.DB, table, name string, tenantID uint) {
	t.Helper()
	var parent model.TenantSystemMenu
	if err := db.Table(table).Where("name = ? AND type = ? AND tenant_id = ?", name, "C", tenantID).
		Order("id desc").First(&parent).Error; err != nil {
		t.Fatalf("%s tenant_id=%d parent: %v", table, tenantID, err)
	}
	if parent.Pid != 0 || parent.Perms != "config/lists" {
		t.Fatalf("%s parent %+v", table, parent)
	}
	var kids []model.TenantSystemMenu
	db.Table(table).Where("pid = ? AND tenant_id = ?", parent.ID, tenantID).Order("id asc").Find(&kids)
	if len(kids) != 3 {
		t.Fatalf("%s tenant_id=%d children=%d want 3", table, tenantID, len(kids))
	}
	want := []string{"添加", "编辑", "删除"}
	for i, name := range want {
		if kids[i].Name != name || kids[i].Type != "A" || kids[i].TenantID != tenantID {
			t.Fatalf("%s child %d %+v want %s tenant_id=%d", table, i, kids[i], name, tenantID)
		}
	}
}
