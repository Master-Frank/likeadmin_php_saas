package gencrud

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"

	"github.com/gin-gonic/gin"
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

func TestPHPNotesLabel(t *testing.T) {
	if phpNotesLabel("对拍生成器", "lists") != "获取对拍生成器列表" {
		t.Fatal(phpNotesLabel("对拍生成器", "lists"))
	}
	if phpNotesLabel("对拍生成器", "add") != "添加对拍生成器" {
		t.Fatal(phpNotesLabel("对拍生成器", "add"))
	}
	if phpNotesLabel("对拍生成器", "detail") != "获取对拍生成器详情" {
		t.Fatal(phpNotesLabel("对拍生成器", "detail"))
	}
	if phpNotesLabel("对拍生成器", "sort") != "" {
		t.Fatal("unknown action")
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

func TestRequireTenantWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sp := &spec{allowed: map[string]bool{"tenant_id": true}}
	plain := &spec{allowed: map[string]bool{"name": true}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	if !requireTenantWrite(c, plain) {
		t.Fatal("tables without tenant_id should pass")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourcePlatform})
	if requireTenantWrite(c, sp) {
		t.Fatal("platform without tenant_id should fail")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourcePlatform, TenantID: 1})
	if !requireTenantWrite(c, sp) {
		t.Fatal("platform with tenant_id should pass")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant})
	if requireTenantWrite(c, sp) {
		t.Fatal("tenant without tenant_id should fail")
	}

	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant, TenantID: 2})
	if !requireTenantWrite(c, sp) {
		t.Fatal("tenant with tenant_id should pass")
	}
}

func TestTableHasColumnNilDB(t *testing.T) {
	if tableHasColumn(nil, "la_user", "tenant_id") {
		t.Fatal("nil db should not claim column")
	}
}

func TestRequiredMsgUsesColumnComment(t *testing.T) {
	sp := &spec{cols: []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1, IsRequired: 1, IsInsert: 1, IsUpdate: 1},
		{ColumnName: "name", ColumnComment: "名称", IsRequired: 1, IsInsert: 1, IsUpdate: 1},
		{ColumnName: "status", ColumnComment: "", IsRequired: 1, IsInsert: 1, IsUpdate: 1},
	}}
	if got := requiredMsg(sp, map[string]any{}, false); got != "名称" {
		t.Fatalf("empty add %q", got)
	}
	if got := requiredMsg(sp, map[string]any{"name": "x"}, false); got != "status" {
		t.Fatalf("missing status %q", got)
	}
	if got := requiredMsg(sp, map[string]any{"name": "x", "status": 1}, false); got != "" {
		t.Fatalf("ok add %q", got)
	}
	if got := requiredMsg(sp, map[string]any{"name": ""}, true); got != "名称" {
		t.Fatalf("empty edit %q", got)
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
