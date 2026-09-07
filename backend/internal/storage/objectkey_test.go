package storage

import (
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
)

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

func TestLocalPublicPath(t *testing.T) {
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = dir
	t.Cleanup(func() { config.C.App.PublicDir = old })

	got, err := localPublicPath("uploads/a.png")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.Abs(filepath.Join(dir, "uploads", "a.png"))
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
	for _, rel := range []string{"uploads/../../../etc/passwd", "..", "../secret", ""} {
		if _, err := localPublicPath(rel); err == nil {
			t.Fatalf("expected reject %q", rel)
		}
	}
	if _, err := localPublicPath(filepath.Join("uploads", strings.Repeat("../", 8)+"etc/passwd")); err == nil {
		t.Fatal("nested traversal")
	}
}
