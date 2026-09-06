package gencrud

import (
	"testing"

	"likeadmin/backend/internal/model"
)

func TestRouteKey(t *testing.T) {
	if RouteKey(model.GenerateTable{Name: "la_config"}) != "config" {
		t.Fatalf("got %s", RouteKey(model.GenerateTable{Name: "la_config"}))
	}
	if RouteKey(model.GenerateTable{Name: "la_config", ClassDir: "setting"}) != "setting.config" {
		t.Fatalf("class_dir %s", RouteKey(model.GenerateTable{Name: "la_config", ClassDir: "setting"}))
	}
}

func TestModuleApp(t *testing.T) {
	if ModuleApp("platform") != "platformapi" || ModuleApp("tenant") != "tenantapi" || ModuleApp("api") != "api" {
		t.Fatalf("module map %s %s %s", ModuleApp("platform"), ModuleApp("tenant"), ModuleApp("api"))
	}
}

func TestValidIdent(t *testing.T) {
	if !validIdent("create_time") || !validIdent("la_pair_gencrud") {
		t.Fatal("valid ident rejected")
	}
	if validIdent("id;drop") || validIdent("a.b") || validIdent("") {
		t.Fatal("unsafe ident accepted")
	}
}

func TestParamName(t *testing.T) {
	if paramName("user.name") != "name" {
		t.Fatal(paramName("user.name"))
	}
	if paramName("name") != "name" {
		t.Fatal(paramName("name"))
	}
}

func TestModelToTable(t *testing.T) {
	if modelToTable(`app\common\model\User`) != "la_user" {
		t.Fatalf("user %s", modelToTable(`app\common\model\User`))
	}
	if modelToTable("ArticleCate") != "la_article_cate" {
		t.Fatalf("cate %s", modelToTable("ArticleCate"))
	}
	if modelToTable("la_user") != "la_user" {
		t.Fatalf("prefixed %s", modelToTable("la_user"))
	}
	if modelToTable("id;drop") != "" {
		t.Fatal("unsafe model")
	}
}

func TestParseRelations(t *testing.T) {
	rels := parseRelations(model.GenerateTable{Relations: `[{"name":"user","model":"User","type":"has_one","local_key":"user_id","foreign_key":"id","label":"nickname"}]`})
	if len(rels) != 1 || rels[0].Table != "la_user" || rels[0].LocalKey != "user_id" || rels[0].Label != "nickname" {
		t.Fatalf("%+v", rels)
	}
	many := parseRelations(model.GenerateTable{Relations: `[{"name":"items","model":"PairItem","type":"has_many","local_key":"id","foreign_key":"pid"}]`})
	if len(many) != 1 || many[0].Type != "has_many" || many[0].Table != "la_pair_item" || many[0].ForeignKey != "pid" {
		t.Fatalf("%+v", many)
	}
}

func TestAttachHasMany(t *testing.T) {
	rows := []map[string]any{{"id": 1, "name": "a"}, {"id": 2, "name": "b"}}
	related := []map[string]any{
		{"id": 11, "pid": 1, "title": "x"},
		{"id": 12, "pid": 1, "title": "y"},
	}
	attachHasMany(rows, related, relSpec{Name: "items", LocalKey: "id", ForeignKey: "pid"})
	first, _ := rows[0]["items"].([]map[string]any)
	if len(first) != 2 {
		t.Fatalf("parent items=%v", rows[0]["items"])
	}
	second, _ := rows[1]["items"].([]map[string]any)
	if len(second) != 0 {
		t.Fatalf("empty has_many=%v", rows[1]["items"])
	}
}

func TestImageCol(t *testing.T) {
	sp := newSpec(model.GenerateTable{Name: "la_pair_gencrud"}, []model.GenerateColumn{
		{ColumnName: "cover", ViewType: "imageSelect"},
		{ColumnName: "name", ViewType: "input"},
	})
	if !isImageCol(sp, "cover") || isImageCol(sp, "name") {
		t.Fatal("image col")
	}
}

func TestTableHasColumnNilDB(t *testing.T) {
	if tableHasColumn(nil, "la_user", "tenant_id") {
		t.Fatal("nil db should not claim column")
	}
}

func TestNewSpecSoftDeleteAndPk(t *testing.T) {
	sp := newSpec(model.GenerateTable{
		Name:   "la_pair_gencrud",
		Delete: `{"type":1,"name":"delete_time"}`,
	}, []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1, IsLists: 1},
		{ColumnName: "name", IsInsert: 1, IsUpdate: 1, IsLists: 1, IsQuery: 1, QueryType: "like"},
		{ColumnName: "delete_time"},
	})
	if sp.pk != "id" || !sp.softDelete || sp.deleteCol != "delete_time" {
		t.Fatalf("%+v", sp)
	}
	if !sp.allowed["name"] || sp.allowed["id;drop"] {
		t.Fatal("allowlist")
	}
}
