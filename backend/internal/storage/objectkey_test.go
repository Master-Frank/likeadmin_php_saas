package storage

import "testing"

func TestObjectKey(t *testing.T) {
	cases := map[string]string{
		"uploads/a.png":                            "uploads/a.png",
		"/uploads/a.png":                           "uploads/a.png",
		"https://cdn.example/uploads/a.png?x=1":    "uploads/a.png",
		"http://127.0.0.1:8091/uploads/file/b.pdf": "uploads/file/b.pdf",
		"cdn.example.com/uploads/c.jpg":            "uploads/c.jpg",
		"":                                         "",
	}
	for in, want := range cases {
		if got := ObjectKey(in); got != want {
			t.Fatalf("ObjectKey(%q)=%q want %q", in, got, want)
		}
	}
}
