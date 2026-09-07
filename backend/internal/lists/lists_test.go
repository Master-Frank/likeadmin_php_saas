package lists

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestIdent(t *testing.T) {
	if Ident("create_time") != "create_time" || Ident("id") != "id" {
		t.Fatal("valid")
	}
	if Ident("id;drop") != "" || Ident("a.b") != "" || Ident("") != "" {
		t.Fatal("invalid")
	}
}

func TestOrderSQL(t *testing.T) {
	q := Query{Field: "name", OrderBy: "asc"}
	if OrderSQL(q, "id desc", nil) != "name asc" {
		t.Fatal(OrderSQL(q, "id desc", nil))
	}
	q.Field = "id;drop"
	if OrderSQL(q, "id desc", nil) != "id desc" {
		t.Fatal("unsafe field")
	}
	q.Field = "name"
	q.OrderBy = "sideways"
	if OrderSQL(q, "id desc", nil) != "id desc" {
		t.Fatal("bad dir")
	}
	q.OrderBy = "desc"
	if OrderSQL(q, "id desc", map[string]bool{"id": true}) != "id desc" {
		t.Fatal("allowlist")
	}
}

func TestParsePageType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSize, oldMax := config.C.Project.Lists.PageSize, config.C.Project.Lists.PageSizeMax
	config.C.Project.Lists.PageSize = 25
	config.C.Project.Lists.PageSizeMax = 25000
	t.Cleanup(func() {
		config.C.Project.Lists.PageSize = oldSize
		config.C.Project.Lists.PageSizeMax = oldMax
	})

	parse := func(raw string) Query {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/lists"+raw, nil)
		return Parse(c)
	}

	q := parse("")
	if q.PageType != 1 || q.PageNo != 1 || q.PageSize != 25 || q.Offset != 0 {
		t.Fatalf("default %+v", q)
	}
	q = parse("?page_type=1&page_no=3&page_size=10")
	if q.PageType != 1 || q.PageNo != 3 || q.PageSize != 10 || q.Offset != 20 {
		t.Fatalf("page %+v", q)
	}
	q = parse("?page_type=0&page_no=3&page_size=10")
	if q.PageType != 0 || q.PageNo != 1 || q.PageSize != 25000 || q.Offset != 0 {
		t.Fatalf("unpaged %+v", q)
	}
}

func TestParseIgnoresJSONPageNo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSize, oldMax := config.C.Project.Lists.PageSize, config.C.Project.Lists.PageSizeMax
	config.C.Project.Lists.PageSize = 25
	config.C.Project.Lists.PageSizeMax = 25000
	t.Cleanup(func() {
		config.C.Project.Lists.PageSize = oldSize
		config.C.Project.Lists.PageSizeMax = oldMax
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/lists?page_no=2&keyword=query", bytes.NewBufferString(`{"page_no":9,"page_size":3,"keyword":"body"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	q := Parse(c)
	if q.PageNo != 2 || q.PageSize != 25 {
		t.Fatalf("paging must ignore JSON body, got page_no=%d page_size=%d", q.PageNo, q.PageSize)
	}
	if Param(q, "keyword") != "body" {
		t.Fatalf("search filters still merge body, got %q", Param(q, "keyword"))
	}
}

func TestParseExportWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldSize, oldMax := config.C.Project.Lists.PageSize, config.C.Project.Lists.PageSizeMax
	config.C.Project.Lists.PageSize = 25
	config.C.Project.Lists.PageSizeMax = 25000
	t.Cleanup(func() {
		config.C.Project.Lists.PageSize = oldSize
		config.C.Project.Lists.PageSizeMax = oldMax
	})

	parse := func(raw string) Query {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/lists"+raw, nil)
		return Parse(c)
	}

	q := parse("?export=2&page_start=2&page_end=4&page_size=10")
	if q.Export != 2 || q.PageType != 1 || q.PageStart != 2 || q.PageEnd != 4 {
		t.Fatalf("meta %+v", q)
	}
	if q.Offset != 10 || q.PageSize != 30 {
		t.Fatalf("window offset=%d size=%d", q.Offset, q.PageSize)
	}

	q = parse("?export=2&page_type=0&page_start=2&page_end=4&page_size=10")
	if q.Offset != 0 || q.PageSize != 25000 {
		t.Fatalf("unpaged export %+v", q)
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/lists?page_size=15", bytes.NewBufferString(`{"export":2,"page_start":2,"page_end":4}`))
	c.Request.Header.Set("Content-Type", "application/json")
	q = Parse(c)
	if q.Export != 0 || q.Offset != 0 || q.PageSize != 15 {
		t.Fatalf("export window must ignore JSON body, got %+v", q)
	}
}

func TestHasParamTreatsZeroAsPresent(t *testing.T) {
	// PHP GET cid=0 is string "0"; "0" == "" is false, so '=' filters apply.
	// JSON body 0 is int; 0 == "" is true and PHP skips — Params still stores "0".
	q := Query{Params: map[string]any{"cid": "0", "type": 0}}
	if !HasParam(q, "cid") || !HasParam(q, "type") {
		t.Fatal("numeric zero must be present")
	}
	if HasParam(q, "missing") || HasParam(Query{Params: map[string]any{"cid": ""}}, "cid") {
		t.Fatal("empty must be absent")
	}
}

func TestParseGETRejectsPOST(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/lists?page_size=1", bytes.NewBufferString(`{"page_no":9}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if _, ok := ParseGET(c); ok {
		t.Fatal("POST lists must fail")
	}
}
