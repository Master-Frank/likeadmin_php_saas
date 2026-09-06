package openapi

import (
	"testing"

	"likeadmin/backend/internal/util"
)

func TestSortPayWayItems(t *testing.T) {
	out := []map[string]any{
		{"id": 3, "sort": 1, "is_default": 0},
		{"id": 1, "sort": 2, "is_default": 0},
		{"id": 2, "sort": 9, "is_default": 1},
		{"id": 4, "sort": 2, "is_default": 0},
	}
	sortPayWayItems(out)
	if util.ToInt(out[0]["id"]) != 2 {
		t.Fatalf("default first %+v", out[0])
	}
	if util.ToInt(out[1]["id"]) != 1 || util.ToInt(out[2]["id"]) != 4 || util.ToInt(out[3]["id"]) != 3 {
		t.Fatalf("sort,id %+v", out)
	}
}
