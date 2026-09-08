package authsvc

import (
	"reflect"
	"testing"
)

func TestBtnAuth(t *testing.T) {
	cases := []struct {
		name      string
		root      bool
		role, all []string
		want      []string
	}{
		{name: "root", root: true, role: []string{"a"}, all: []string{"a", "b"}, want: []string{"*"}},
		{name: "all covered", role: []string{"a", "b"}, all: []string{"b", "a"}, want: []string{"*"}},
		{name: "partial", role: []string{"a"}, all: []string{"a", "b"}, want: []string{"a"}},
		{name: "none", role: nil, all: []string{"a"}, want: []string{}},
		{name: "empty catalogs", role: nil, all: nil, want: []string{"*"}},
		{name: "dedupe blanks", role: []string{"a", "", "a"}, all: []string{"a", "b"}, want: []string{"a"}},
	}
	for _, tc := range cases {
		got := BtnAuth(tc.root, tc.role, tc.all)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: got %#v want %#v", tc.name, got, tc.want)
		}
	}
}
