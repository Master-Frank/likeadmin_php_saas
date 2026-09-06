package util

import "testing"

func TestCreatePasswordMatchesPHP(t *testing.T) {
	// PHP: md5(salt + md5(plaintext + salt))
	got := CreatePassword("likeadmin", "likeadmin")
	want := MD5("likeadmin" + MD5("likeadmin"+"likeadmin"))
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	if len(got) != 32 {
		t.Fatalf("md5 length %d", len(got))
	}
}

func TestLinearToTree(t *testing.T) {
	data := []map[string]any{
		{"id": 1, "pid": 0, "name": "a"},
		{"id": 2, "pid": 1, "name": "b"},
		{"id": 3, "pid": 0, "name": "c"},
	}
	tree := LinearToTree(data, "children", "id", "pid", 0)
	if len(tree) != 2 {
		t.Fatalf("roots %d", len(tree))
	}
	child, _ := tree[0]["children"].([]map[string]any)
	if tree[0]["id"] == 1 && len(child) != 1 {
		t.Fatalf("child missing")
	}
}
