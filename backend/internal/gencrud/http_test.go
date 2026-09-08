package gencrud

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const runtimeTable = "la_go_gencrud_rt"

func initGencrudDB(t *testing.T) bool {
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
	if bootstrap.DB != nil {
		tenantdb.Register(bootstrap.DB)
	}
	return bootstrap.DB != nil
}

func seedRuntimeTable(t *testing.T) uint {
	t.Helper()
	db := bootstrap.DB
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS ` + runtimeTable + ` (
		id int unsigned NOT NULL AUTO_INCREMENT,
		name varchar(64) NOT NULL DEFAULT '',
		cover varchar(255) NOT NULL DEFAULT '',
		tags varchar(255) NOT NULL DEFAULT '',
		body text,
		status tinyint NOT NULL DEFAULT 1,
		create_time int NOT NULL DEFAULT 0,
		update_time int DEFAULT NULL,
		delete_time int DEFAULT NULL,
		PRIMARY KEY (id)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`).Error; err != nil {
		t.Fatal(err)
	}
	db.Exec("DELETE FROM la_generate_column WHERE table_id IN (SELECT id FROM la_generate_table WHERE table_name = ?)", runtimeTable)
	db.Exec("DELETE FROM la_generate_table WHERE table_name = ?", runtimeTable)
	db.Exec("DELETE FROM " + runtimeTable)

	tbl := model.GenerateTable{
		Name: runtimeTable, TableComment: "Go运行时生成器", TemplateType: 0,
		Author: "likeadmin", GenerateType: 1, ModuleName: "platform",
		ClassComment: "Go运行时生成器",
		Menu:         `{"pid":0,"type":0,"name":"Go运行时生成器"}`,
		Delete:       `{"type":1,"name":"delete_time"}`,
		Tree:         `{}`, Relations: `[]`,
		CreateTime: time.Now().Unix(),
	}
	if err := db.Create(&tbl).Error; err != nil {
		t.Fatal(err)
	}
	cols := []model.GenerateColumn{
		{TableID: tbl.ID, ColumnName: "id", ColumnComment: "主键", ColumnType: "int", IsPk: 1, IsLists: 1, ViewType: "input"},
		{TableID: tbl.ID, ColumnName: "name", ColumnComment: "名称", ColumnType: "string", IsRequired: 1, IsInsert: 1, IsUpdate: 1, IsLists: 1, IsQuery: 1, QueryType: "like", ViewType: "input"},
		{TableID: tbl.ID, ColumnName: "cover", ColumnComment: "封面", ColumnType: "string", IsInsert: 1, IsUpdate: 1, IsLists: 1, ViewType: "imageSelect"},
		{TableID: tbl.ID, ColumnName: "tags", ColumnComment: "标签", ColumnType: "string", IsInsert: 1, IsUpdate: 1, IsLists: 1, ViewType: "checkbox"},
		{TableID: tbl.ID, ColumnName: "body", ColumnComment: "内容", ColumnType: "text", IsInsert: 1, IsUpdate: 1, IsLists: 1, ViewType: "editor"},
		{TableID: tbl.ID, ColumnName: "status", ColumnComment: "状态", ColumnType: "int", IsInsert: 1, IsUpdate: 1, IsLists: 1, ViewType: "radio"},
		{TableID: tbl.ID, ColumnName: "create_time", ColumnComment: "创建时间", ColumnType: "int", IsLists: 1, ViewType: "datetime"},
		{TableID: tbl.ID, ColumnName: "update_time", ColumnComment: "更新时间", ColumnType: "int", IsLists: 1, ViewType: "datetime"},
	}
	if err := db.Create(&cols).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM la_generate_column WHERE table_id = ?", tbl.ID)
		db.Exec("DELETE FROM la_generate_table WHERE id = ?", tbl.ID)
		db.Exec("DROP TABLE IF EXISTS " + runtimeTable)
	})
	return tbl.ID
}

func gencrudCtx(method, action, rawURL string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var r *http.Request
	if body != nil {
		b, _ := json.Marshal(body)
		r = httptest.NewRequest(method, rawURL, bytes.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, rawURL, nil)
	}
	r.Host = "pair1.likeadmin.test"
	c.Request = r
	ctxutil.Set(c, &ctxutil.RequestMeta{
		App: "platformapi", Controller: "go_gencrud_rt", Action: action,
		Source: ctxutil.SourcePlatform,
	})
	return c, w
}

func decodeWrap(t *testing.T, w *httptest.ResponseRecorder) response.Body {
	t.Helper()
	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	return wrap
}

func TestGencrudHTTPRuntimeCRUD(t *testing.T) {
	if !initGencrudDB(t) {
		t.Skip("no database")
	}
	seedRuntimeTable(t)
	if !Match("platformapi", "go_gencrud_rt", "lists") {
		t.Fatal("Match should resolve generate_type=1 route")
	}

	c, w := gencrudCtx(http.MethodPost, "add", "/platformapi/go_gencrud_rt/add", map[string]any{
		"name":   "rt-row",
		"cover":  "http://pair1.likeadmin.test/uploads/cover.png",
		"tags":   []any{"a", "b"},
		"body":   `<p><img src="http://pair1.likeadmin.test/uploads/ed.png"></p>`,
		"status": 1,
	})
	Handle(c)
	if wrap := decodeWrap(t, w); wrap.Code != 1 || wrap.Msg != "添加成功" {
		t.Fatalf("add %+v body=%s", wrap, w.Body.String())
	}

	var stored map[string]any
	if err := bootstrap.DB.Table(runtimeTable).Where("name = ? AND delete_time IS NULL", "rt-row").Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	normalizeMap(stored)
	if stored["cover"] != "uploads/cover.png" {
		t.Fatalf("stored cover %v", stored["cover"])
	}
	if stored["tags"] != "a,b" {
		t.Fatalf("stored tags %v", stored["tags"])
	}
	if stored["body"] != `<p><img src="uploads/ed.png"></p>` {
		t.Fatalf("stored body %v", stored["body"])
	}
	id := uint(util.ToInt(stored["id"]))
	if id == 0 {
		t.Fatalf("id %v", stored["id"])
	}

	c, w = gencrudCtx(http.MethodGet, "lists", "/platformapi/go_gencrud_rt/lists?name=rt-row", nil)
	Handle(c)
	wrap := decodeWrap(t, w)
	if wrap.Code != 1 {
		t.Fatalf("lists %+v", wrap)
	}
	data, _ := wrap.Data.(map[string]any)
	lists, _ := data["lists"].([]any)
	if len(lists) != 1 {
		t.Fatalf("lists count=%d data=%v", len(lists), wrap.Data)
	}
	row, _ := lists[0].(map[string]any)
	if row["cover"] != "http://pair1.likeadmin.test/uploads/cover.png" {
		t.Fatalf("lists cover %v", row["cover"])
	}
	if row["tags"] != "a,b" {
		t.Fatalf("lists tags %v", row["tags"])
	}
	if row["body"] != `<p><img src="http://pair1.likeadmin.test/uploads/ed.png"></p>` {
		t.Fatalf("lists body %v", row["body"])
	}

	c, w = gencrudCtx(http.MethodGet, "detail", "/platformapi/go_gencrud_rt/detail?id="+strconv.FormatUint(uint64(id), 10), nil)
	Handle(c)
	wrap = decodeWrap(t, w)
	if wrap.Code != 1 {
		t.Fatalf("detail %+v", wrap)
	}
	detail, _ := wrap.Data.(map[string]any)
	if detail["name"] != "rt-row" {
		t.Fatalf("detail name %v", detail["name"])
	}

	c, w = gencrudCtx(http.MethodPost, "edit", "/platformapi/go_gencrud_rt/edit", map[string]any{
		"id": id, "name": "rt-edit", "cover": "uploads/cover2.png", "tags": "x", "body": "<p>ok</p>", "status": 0,
	})
	Handle(c)
	if wrap = decodeWrap(t, w); wrap.Code != 1 {
		t.Fatalf("edit %+v", wrap)
	}

	c, w = gencrudCtx(http.MethodPost, "delete", "/platformapi/go_gencrud_rt/delete", map[string]any{"id": id})
	Handle(c)
	if wrap = decodeWrap(t, w); wrap.Code != 1 {
		t.Fatalf("delete %+v", wrap)
	}
	var left int64
	bootstrap.DB.Table(runtimeTable).Where("id = ? AND delete_time IS NULL", id).Count(&left)
	if left != 0 {
		t.Fatalf("soft delete left=%d", left)
	}
}

func TestTreeAncestorStaysOnOwnTenant(t *testing.T) {
	if !initGencrudDB(t) {
		t.Skip("no database")
	}
	const table = "la_go_tree_rt"
	db := bootstrap.DB
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS " + table).Error })
	_ = db.Exec("DROP TABLE IF EXISTS " + table).Error
	if err := db.Exec(`CREATE TABLE ` + table + ` (
		id int unsigned NOT NULL,
		pid int unsigned NOT NULL DEFAULT 0,
		tenant_id int unsigned NOT NULL DEFAULT 0,
		PRIMARY KEY (id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + table + " (id, pid, tenant_id) VALUES (10, 20, 990013), (20, 10, 990014)").Error; err != nil {
		t.Fatal(err)
	}
	sp := &spec{
		table: model.GenerateTable{Name: table},
		pk:    "id", tree: true, treeID: "id", treePID: "pid",
		allowed: map[string]bool{"id": true, "pid": true, "tenant_id": true},
	}
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{Source: ctxutil.SourceTenant, TenantID: 990013})
	if isTreeAncestor(c, sp, 20, 10) {
		t.Fatal("must not walk another tenant's pid chain")
	}
	if msg := treeCycleMsg(c, sp, 10, map[string]any{"pid": 20}); msg != "" {
		t.Fatalf("cross-tenant pid should not report cycle: %s", msg)
	}
}

func TestAttachRelationsUsesShardTable(t *testing.T) {
	if !initGencrudDB(t) {
		t.Skip("no database")
	}
	const sn = "t990007"
	const tid uint = 990007
	db := bootstrap.DB
	parent := "la_go_rel_rt"
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS " + parent).Error
		_ = db.Exec("DROP TABLE IF EXISTS la_user_" + sn).Error
	})
	if err := db.Exec("DROP TABLE IF EXISTS la_user_" + sn).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE la_user_" + sn + " LIKE la_user").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS ` + parent + ` (
		id int unsigned NOT NULL AUTO_INCREMENT,
		user_id int unsigned NOT NULL DEFAULT 0,
		name varchar(64) NOT NULL DEFAULT '',
		PRIMARY KEY (id)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO la_user_"+sn+" (id, nickname, tenant_id, create_time) VALUES (1, 'shard-nick', ?, ?)",
		tid, time.Now().Unix()).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO " + parent + " (id, user_id, name) VALUES (1, 1, 'row')").Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctxutil.Set(c, &ctxutil.RequestMeta{TenantID: tid, TenantSN: sn, Tactics: 1})

	sp := newSpec(model.GenerateTable{
		Name:      parent,
		Relations: `[{"name":"user","model":"User","type":"has_one","local_key":"user_id","foreign_key":"id","label":"nickname"}]`,
	}, []model.GenerateColumn{
		{ColumnName: "id", IsPk: 1, IsLists: 1},
		{ColumnName: "user_id", IsLists: 1},
		{ColumnName: "name", IsLists: 1},
	})
	rows := []map[string]any{{"id": 1, "user_id": 1, "name": "row"}}
	attachRelations(c, sp, rows)
	user, _ := rows[0]["user"].(map[string]any)
	if user == nil || util.ToString(user["nickname"]) != "shard-nick" {
		t.Fatalf("want shard user, got %+v", rows[0]["user"])
	}
	if util.ToString(rows[0]["user_name"]) != "shard-nick" {
		t.Fatalf("user_name %v", rows[0]["user_name"])
	}
}
