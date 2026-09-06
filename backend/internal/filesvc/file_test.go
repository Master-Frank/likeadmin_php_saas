package filesvc

import "testing"

func TestRewriteContentDomains(t *testing.T) {
	in := `<p><img src="uploads/images/a.png"><video src="uploads/video/b.mp4"></video><img src="https://cdn.example/c.png"></p>`
	got := rewriteContent("http://pair1.likeadmin.test/", in)
	want := `<p><img src="http://pair1.likeadmin.test/uploads/images/a.png"><video src="http://pair1.likeadmin.test/uploads/video/b.mp4"></video><img src="https://cdn.example/c.png"></p>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if rewriteContent("", in) != in {
		t.Fatal("empty domain should keep content")
	}
}
