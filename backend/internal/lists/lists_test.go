package lists

import "testing"

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
