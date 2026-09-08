package platformapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/generator"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
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

func TestPHPJSONAssoc(t *testing.T) {
	if _, ok := phpJSONAssoc("{}").([]any); !ok {
		t.Fatalf("empty object must become list, got %T", phpJSONAssoc("{}"))
	}
	if _, ok := phpJSONAssoc("[]").([]any); !ok {
		t.Fatalf("array %T", phpJSONAssoc("[]"))
	}
	m, ok := phpJSONAssoc(`{"pid":0,"type":0,"name":"x"}`).(map[string]any)
	if !ok || m["name"] != "x" {
		t.Fatalf("object %v", phpJSONAssoc(`{"pid":0,"type":0,"name":"x"}`))
	}
}

func TestFormatGenerateTableListKeys(t *testing.T) {
	row := formatGenerateTableList(model.GenerateTable{
		ID: 1, Name: "la_config", TableComment: "cfg", Menu: `{"pid":0,"type":0,"name":"n"}`,
		Delete: `{"type":0,"name":"delete_time"}`, Tree: `{}`, Relations: `[]`,
	})
	for _, k := range []string{"id", "table_name", "table_comment", "template_type", "template_type_desc",
		"generate_type", "module_name", "class_dir", "class_comment", "admin_id", "author", "remark",
		"menu", "delete", "tree", "relations", "create_time", "update_time"} {
		if _, ok := row[k]; !ok {
			t.Fatalf("missing %s", k)
		}
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

func TestValidModelModule(t *testing.T) {
	if !validModelModule("common") || !validModelModule("tenant") {
		t.Fatal("valid modules rejected")
	}
	if validModelModule("") || validModelModule("../common") || validModelModule("a/b") || validModelModule("a\\b") {
		t.Fatal("unsafe module accepted")
	}
}

func TestPhysicalTableName(t *testing.T) {
	if got := physicalTableName("la_pair_gencrud"); got != "la_pair_gencrud" {
		t.Fatalf("prefixed: %s", got)
	}
	if got := physicalTableName("pair_gencrud"); got != "la_pair_gencrud" {
		t.Fatalf("stripped: %s", got)
	}
	if got := physicalTableName("  config  "); got != "la_config" {
		t.Fatalf("trim: %s", got)
	}
}

func TestGeneratorDownloadRejectsCacheMiss(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := generator.RuntimeDir()
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	name := "curd-miss-990022.zip"
	zipPath := filepath.Join(root, name)
	if err := os.WriteFile(zipPath, []byte("PK\x03\x04miss"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(zipPath) })

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tools.generator/download?file="+name, nil)
	GeneratorDownload(c)
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code == 1 || wrap.Msg != "请重新生成代码" {
		t.Fatalf("cache-miss %+v body=%s", wrap, w.Body.String())
	}

	cache.Set("curd_file_name"+name, name, time.Hour)
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tools.generator/download?file="+name, nil)
	GeneratorDownload(c2)
	if w2.Code != http.StatusOK || !bytes.Contains(w2.Body.Bytes(), []byte("PK")) {
		t.Fatalf("cache-hit status=%d body=%q", w2.Code, w2.Body.String())
	}
	if _, ok := cache.Get("curd_file_name" + name); ok {
		t.Fatal("PHP download must consume the cache token")
	}

	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tools.generator/download?file="+name, nil)
	GeneratorDownload(c3)
	var wrap2 response.Body
	if err := json.Unmarshal(w3.Body.Bytes(), &wrap2); err != nil {
		t.Fatalf("second json %s: %v", w3.Body.String(), err)
	}
	if wrap2.Msg != "请重新生成代码" {
		t.Fatalf("second download %+v", wrap2)
	}
}

func TestGeneratorDownloadMissingFileFollowsPHP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tools.generator/download", nil)
	GeneratorDownload(c)
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Msg != "下载失败" {
		t.Fatalf("missing file param %+v", wrap)
	}

	gone := "curd-gone-990023.zip"
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tools.generator/download?file="+gone, nil)
	GeneratorDownload(c2)
	var wrap2 response.Body
	if err := json.Unmarshal(w2.Body.Bytes(), &wrap2); err != nil {
		t.Fatalf("json2 %s: %v", w2.Body.String(), err)
	}
	if wrap2.Msg != "请重新生成代码" {
		t.Fatalf("cache-miss missing file %+v", wrap2)
	}

	cache.Set("curd_file_name"+gone, gone, time.Hour)
	t.Cleanup(func() { cache.Del("curd_file_name" + gone) })
	w3 := httptest.NewRecorder()
	c3, _ := gin.CreateTestContext(w3)
	c3.Request = httptest.NewRequest(http.MethodGet, "/platformapi/tools.generator/download?file="+gone, nil)
	GeneratorDownload(c3)
	var wrap3 response.Body
	if err := json.Unmarshal(w3.Body.Bytes(), &wrap3); err != nil {
		t.Fatalf("json3 %s: %v", w3.Body.String(), err)
	}
	if wrap3.Msg != "下载失败" {
		t.Fatalf("cache-hit missing file %+v", wrap3)
	}
}

func TestGeneratorDownloadURL(t *testing.T) {
	got := generatorDownloadURL("http://pair1.likeadmin.test", "platformapi", "curd-20260101120000.zip")
	want := "http://pair1.likeadmin.test/platformapi/tools.generator/download?file=curd-20260101120000.zip"
	if got != want {
		t.Fatalf("%s", got)
	}
	if generatorDownloadURL("http://h", "", "a.zip") != "http://h/platformapi/tools.generator/download?file=a.zip" {
		t.Fatal("empty app defaults to platformapi")
	}
	if generatorDownloadURL("http://h", "tenantapi", "a.zip") != "http://h/tenantapi/tools.generator/download?file=a.zip" {
		t.Fatal("tenant app")
	}
}

func TestScanPHPModelsModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "User.php"), []byte("<?php"), 0644); err != nil {
		t.Fatal(err)
	}
	got := scanPHPModels(dir, "tenant")
	if len(got) != 1 || got[0] != `\app\tenant\model\User` {
		t.Fatalf("got %v", got)
	}
}

func TestReplaceGenerateColumnsRollsBack(t *testing.T) {
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if bootstrap.DB == nil {
		if err := bootstrap.Init(cfg); err != nil {
			t.Skip(err)
		}
	}
	if bootstrap.DB == nil {
		t.Skip("no database")
	}
	now := util.NowUnix()
	gt := model.GenerateTable{
		Name: "la_pair_missing_sync_61", TableComment: "rollback", Author: "pair",
		ModuleName: "platform", Menu: `{"pid":0,"type":0,"name":"n"}`,
		Delete: `{"type":0,"name":"delete_time"}`, Tree: `{}`, Relations: `[]`,
		CreateTime: now, UpdateTime: &now,
	}
	if err := bootstrap.DB.Create(&gt).Error; err != nil {
		t.Fatal(err)
	}
	col := model.GenerateColumn{
		TableID: gt.ID, ColumnName: "keep_me", ColumnComment: "keep",
		ColumnType: "string", QueryType: "=", ViewType: "input",
		CreateTime: now, UpdateTime: &now,
	}
	if err := bootstrap.DB.Create(&col).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("table_id = ?", gt.ID).Delete(&model.GenerateColumn{})
		bootstrap.DB.Delete(&gt)
	})
	if err := replaceGenerateColumns(bootstrap.DB, gt.ID, "la_pair_missing_sync_61"); err == nil {
		t.Fatal("missing table should fail like PHP getFields")
	}
	var n int64
	bootstrap.DB.Model(&model.GenerateColumn{}).Where("id = ?", col.ID).Count(&n)
	if n != 1 {
		t.Fatal("PHP syncColumn transaction must keep old columns on failure")
	}
}
