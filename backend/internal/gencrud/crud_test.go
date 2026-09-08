package gencrud

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func TestRouteKey(t *testing.T) {
	if RouteKey(model.GenerateTable{Name: "la_config"}) != "config" {
		t.Fatalf("got %s", RouteKey(model.GenerateTable{Name: "la_config"}))
	}
	if RouteKey(model.GenerateTable{Name: "la_config", ClassDir: "setting"}) != "setting.config" {
		t.Fatalf("class_dir %s", RouteKey(model.GenerateTable{Name: "la_config", ClassDir: "setting"}))
	}
	if RouteKey(model.GenerateTable{Name: "la_config", ClassDir: "Setting"}) != "setting.config" {
		t.Fatalf("class_dir case %s", RouteKey(model.GenerateTable{Name: "la_config", ClassDir: "Setting"}))
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
	emptyType := parseRelations(model.GenerateTable{Relations: `[{"name":"owner","model":"User","local_key":"id","foreign_key":"user_id"}]`})
	if len(emptyType) != 0 {
		t.Fatalf("empty type must skip like PHP: %+v", emptyType)
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
		{ColumnName: "image", ViewType: "input"},
		{ColumnName: "attach", ViewType: "file"},
	})
	if !isImageCol(sp, "cover") || isImageCol(sp, "name") {
		t.Fatal("image col")
	}
	if !isImageCol(sp, "image") {
		t.Fatal("PHP BaseModel image column")
	}
	if !isImageCol(sp, "attach") {
		t.Fatal("file view_type")
	}
}

func TestJoinCheckbox(t *testing.T) {
	if got := joinCheckbox([]any{"a", "b", ""}); got != "a,b" {
		t.Fatalf("slice %q", got)
	}
	if got := joinCheckbox([]string{"x", "y"}); got != "x,y" {
		t.Fatalf("strings %q", got)
	}
	if got := joinCheckbox("already,joined"); got != "already,joined" {
		t.Fatalf("string %q", got)
	}
}

func TestRequiredMsgEmptyCheckbox(t *testing.T) {
	sp := &spec{cols: []model.GenerateColumn{
		{ColumnName: "tags", ColumnComment: "标签", ViewType: "checkbox", IsRequired: 1, IsInsert: 1, IsUpdate: 1},
	}}
	if got := requiredMsg(sp, map[string]any{"tags": []any{}}, false); got != "标签" {
		t.Fatalf("empty checkbox %q", got)
	}
	if got := requiredMsg(sp, map[string]any{"tags": []any{"a"}}, false); got != "" {
		t.Fatalf("filled checkbox %q", got)
	}
}

func TestWriteDataFileAndCheckbox(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.Host = "pair1.likeadmin.test"

	sp := newSpec(model.GenerateTable{Name: "la_go_gencrud_rt"}, []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1},
		{ColumnName: "name", ViewType: "input", IsInsert: 1, IsUpdate: 1},
		{ColumnName: "cover", ViewType: "imageSelect", IsInsert: 1, IsUpdate: 1},
		{ColumnName: "tags", ViewType: "checkbox", IsInsert: 1, IsUpdate: 1},
		{ColumnName: "body", ViewType: "editor", IsInsert: 1, IsUpdate: 1},
		{ColumnName: "image", ViewType: "input", IsInsert: 1, IsUpdate: 1},
	})
	data := writeData(c, sp, map[string]any{
		"name":  "n1",
		"cover": "http://pair1.likeadmin.test/uploads/a.png",
		"tags":  []any{"red", "blue"},
		"body":  `<p><img src="http://pair1.likeadmin.test/uploads/c.png"></p>`,
		"image": "http://pair1.likeadmin.test/uploads/b.png",
	}, false)
	if data["cover"] != "uploads/a.png" {
		t.Fatalf("cover %v", data["cover"])
	}
	if data["image"] != "uploads/b.png" {
		t.Fatalf("image %v", data["image"])
	}
	if data["tags"] != "red,blue" {
		t.Fatalf("tags %v", data["tags"])
	}
	if data["body"] != `<p><img src="uploads/c.png"></p>` {
		t.Fatalf("body %v", data["body"])
	}
}

func TestSelectColsRespectsIsLists(t *testing.T) {
	sp := newSpec(model.GenerateTable{Name: "la_pair_gencrud"}, []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1, IsLists: 1},
		{ColumnName: "name", IsLists: 1},
		{ColumnName: "create_time", IsLists: 0, ViewType: "datetime"},
		{ColumnName: "update_time", IsLists: 0, ViewType: "datetime"},
		{ColumnName: "secret", IsLists: 0},
	})
	got := selectCols(sp)
	joined := strings.Join(got, ",")
	if joined != "id,name" {
		t.Fatalf("selectCols=%v", got)
	}

	sp = newSpec(model.GenerateTable{
		Name: "la_pair_gencrud", TemplateType: 1,
		Tree: `{"tree_id":"id","tree_pid":"pid"}`,
	}, []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1, IsLists: 1},
		{ColumnName: "pid", IsLists: 0},
		{ColumnName: "name", IsLists: 1},
	})
	got = selectCols(sp)
	joined = strings.Join(got, ",")
	if joined != "id,name,pid" {
		t.Fatalf("tree selectCols=%v", got)
	}
}

func TestFormatRowKeepsUnixDatetime(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	sp := newSpec(model.GenerateTable{Name: "la_go_gencrud_rt"}, []model.GenerateColumn{
		{ColumnName: "create_time", ViewType: "datetime", IsLists: 1},
		{ColumnName: "event_time", ViewType: "datetime", IsLists: 1},
		{ColumnName: "name", ViewType: "input", IsLists: 1},
	})
	out := formatRow(c, sp, map[string]any{
		"create_time": int64(1717200000),
		"event_time":  []byte("1717203600"),
		"name":        "n1",
	})
	if out["create_time"] != int64(1717200000) {
		t.Fatalf("create_time %T %v", out["create_time"], out["create_time"])
	}
	if out["event_time"] != int64(1717203600) {
		t.Fatalf("event_time %T %v", out["event_time"], out["event_time"])
	}
	if out["name"] != "n1" {
		t.Fatalf("name %v", out["name"])
	}
}

func TestFormatRowFileAndEditor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Host = "pair1.likeadmin.test"

	sp := newSpec(model.GenerateTable{Name: "la_go_gencrud_rt"}, []model.GenerateColumn{
		{ColumnName: "cover", ViewType: "imageSelect", IsLists: 1},
		{ColumnName: "body", ViewType: "editor", IsLists: 1},
		{ColumnName: "name", ViewType: "input", IsLists: 1},
	})
	out := formatRow(c, sp, map[string]any{
		"cover": "uploads/a.png",
		"body":  `<p><img src="uploads/c.png"></p>`,
		"name":  "n1",
	})
	if out["cover"] != "http://pair1.likeadmin.test/uploads/a.png" {
		t.Fatalf("cover %v", out["cover"])
	}
	if out["body"] != `<p><img src="http://pair1.likeadmin.test/uploads/c.png"></p>` {
		t.Fatalf("body %v", out["body"])
	}
	if out["name"] != "n1" {
		t.Fatalf("name %v", out["name"])
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

func TestCoerceColumnValueDatetime(t *testing.T) {
	col := model.GenerateColumn{ColumnName: "event_time", ColumnType: "int", ViewType: "datetime"}
	got := coerceColumnValue(col, "2024-06-01 10:00:00")
	want := util.ParseDateTime("2024-06-01 10:00:00")
	if got != want || want == 0 {
		t.Fatalf("datetime string => %v want %d", got, want)
	}
	if coerceColumnValue(col, want) != want {
		t.Fatalf("unix passthrough %v", coerceColumnValue(col, want))
	}
	plain := model.GenerateColumn{ColumnName: "name", ColumnType: "string", ViewType: "input"}
	if coerceColumnValue(plain, "2024-06-01 10:00:00") != "2024-06-01 10:00:00" {
		t.Fatal("non-datetime should stay string")
	}
}

func TestParseSearchTimeRange(t *testing.T) {
	start, end, ok := parseSearchTimeRange("2024-06-01", "2024-06-02")
	if !ok || start != util.ParseDateTime("2024-06-01") || end != util.ParseDateTime("2024-06-02") {
		t.Fatalf("range %d %d ok=%v", start, end, ok)
	}
	if _, _, ok := parseSearchTimeRange("", "2024-06-02"); ok {
		t.Fatal("empty start")
	}
	if _, _, ok := parseSearchTimeRange("nope", "2024-06-02"); ok {
		t.Fatal("bad start")
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

func TestNewSpecSoftDeleteWithoutDeleteColumn(t *testing.T) {
	sp := newSpec(model.GenerateTable{
		Name:   "la_pair_gencrud",
		Delete: `{"type":1,"name":"delete_time"}`,
	}, []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1},
		{ColumnName: "name"},
	})
	if !sp.softDelete || sp.deleteCol != "delete_time" {
		t.Fatalf("soft delete from delete.type=1: %+v", sp)
	}
	if sp.allowed["delete_time"] {
		t.Fatal("delete_time is not a generate_column")
	}
}
