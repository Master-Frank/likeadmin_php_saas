package filesvc

import (
	"strings"
	"testing"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
)

func TestGetImageAttrEmpty(t *testing.T) {
	if GetImageAttr(nil, "") != "" || GetImageAttr(nil, "   ") != "" {
		t.Fatal("empty image must stay empty")
	}
	if Format("http://host", "") != "http://host/" {
		t.Fatal("Format empty still prefixes domain")
	}
}

func TestUploadCateOKZero(t *testing.T) {
	if UploadCateOK(nil, nil, 0, 1) != "" {
		t.Fatal("cid 0 should skip lookup")
	}
	if UploadCateOK(nil, nil, 9, 1) != "文件分类不存在" {
		t.Fatal("missing db should reject cid")
	}
}

func TestApplyFileCIDMissingCid(t *testing.T) {
	if ApplyFileCID(nil, nil, map[string]any{}, 7) != nil {
		t.Fatal("missing cid should leave db unchanged")
	}
	if ApplyFileCID(nil, nil, map[string]any{"cid": ""}, 7) != nil {
		t.Fatal("empty cid should leave db unchanged")
	}
}

func TestFileIDsExistEmpty(t *testing.T) {
	if FileIDsExist(nil, nil) || FileIDsExist(nil, []uint{}) {
		t.Fatal("empty ids")
	}
	if FileIDsExist(nil, []uint{0, 1}) {
		t.Fatal("zero id")
	}
}

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

func TestClearContentDomains(t *testing.T) {
	in := `<p><img src="http://pair1.likeadmin.test/uploads/images/a.png"><video src="http://pair1.likeadmin.test/uploads/video/b.mp4"></video><img src="uploads/keep.png"></p>`
	got := mapMediaSrc(in, func(src string) string {
		return strings.ReplaceAll(src, "http://pair1.likeadmin.test/", "")
	})
	want := `<p><img src="uploads/images/a.png"><video src="uploads/video/b.mp4"></video><img src="uploads/keep.png"></p>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStorageCache(t *testing.T) {
	cache.Del("STORAGE_DEFAULT")
	cache.Del("STORAGE_ENGINE")
	t.Cleanup(func() {
		cache.Del("STORAGE_DEFAULT")
		cache.Del("STORAGE_ENGINE")
	})
	cache.Set("STORAGE_DEFAULT", "qiniu", 0)
	cache.Set("STORAGE_ENGINE", map[string]any{"domain": "https://cdn.example/"}, 0)
	if storageDefault(nil) != "qiniu" {
		t.Fatalf("default=%s", storageDefault(nil))
	}
	eng := storageEngine(nil, "qiniu")
	if eng == nil || eng["domain"] != "https://cdn.example/" {
		t.Fatalf("engine=%v", eng)
	}
	if got := GetFileURL(nil, "uploads/a.png"); got != "https://cdn.example/uploads/a.png" {
		t.Fatalf("url=%s", got)
	}
}

func TestPublicPath(t *testing.T) {
	old := config.C.App.PublicDir
	t.Cleanup(func() { config.C.App.PublicDir = old })
	config.C.App.PublicDir = "/var/www/public"
	if got := PublicPath("uploads/a.png"); got != "/var/www/public/uploads/a.png" {
		t.Fatalf("%s", got)
	}
	if got := PublicPath("/uploads/a.png"); got != "/var/www/public/uploads/a.png" {
		t.Fatalf("%s", got)
	}
	if got := PublicPath(""); got != "/var/www/public/" {
		t.Fatalf("%s", got)
	}
}
