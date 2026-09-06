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
