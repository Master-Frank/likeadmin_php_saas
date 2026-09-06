package openapi

import (
	"likeadmin/backend/internal/model"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDecoratePageValueEmpty(t *testing.T) {
	v := decoratePageValue(model.DecoratePage{})
	arr, ok := v.([]any)
	if !ok || len(arr) != 0 {
		t.Fatalf("empty page should be [], got %T %#v", v, v)
	}
}

func TestDecoratePageValuePresent(t *testing.T) {
	page := decoratePageMap(model.DecoratePage{ID: 3, Type: 1, Name: "首页"})
	if page["id"] != uint(3) || page["name"] != "首页" {
		t.Fatalf("%+v", page)
	}
	v := decoratePageValue(model.DecoratePage{ID: 3, Type: 1, Name: "首页"})
	m, ok := v.(gin.H)
	if !ok {
		t.Fatalf("present page should be object, got %T", v)
	}
	if m["name"] != "首页" {
		t.Fatalf("%+v", m)
	}
}
