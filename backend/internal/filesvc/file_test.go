package filesvc

import (
	"net/http/httptest"
	"strings"
	"testing"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
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

func TestClearContentDomainsSingleQuote(t *testing.T) {
	in := `<p><img src='http://pair1.likeadmin.test/uploads/images/a.png'><video src='http://pair1.likeadmin.test/uploads/video/b.mp4'></video></p>`
	got := imgSrcSQRe.ReplaceAllStringFunc(in, func(m string) string {
		return rewriteMediaSrc(imgSrcSQRe, m, func(src string) string {
			return strings.ReplaceAll(src, "http://pair1.likeadmin.test/", "")
		})
	})
	want := `<p><img src='uploads/images/a.png'><video src='http://pair1.likeadmin.test/uploads/video/b.mp4'></video></p>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestClearContentDomainsImgOnly(t *testing.T) {
	in := `<p><img src="http://pair1.likeadmin.test/uploads/images/a.png"><video src="http://pair1.likeadmin.test/uploads/video/b.mp4"></video></p>`
	got := imgSrcRe.ReplaceAllStringFunc(in, func(m string) string {
		return rewriteMediaSrc(imgSrcRe, m, func(src string) string {
			return strings.ReplaceAll(src, "http://pair1.likeadmin.test/", "")
		})
	})
	want := `<p><img src="uploads/images/a.png"><video src="http://pair1.likeadmin.test/uploads/video/b.mp4"></video></p>`
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
	if got := SetFileURL(nil, "https://cdn.example/uploads/a.png"); got != "uploads/a.png" {
		t.Fatalf("set url=%s", got)
	}
}

func TestStorageCacheTenantIsolation(t *testing.T) {
	cache.Del("STORAGE_DEFAULT")
	cache.Del("STORAGE_ENGINE")
	cache.Del("STORAGE_DEFAULT_1")
	cache.Del("STORAGE_ENGINE_1")
	cache.Del("STORAGE_DEFAULT_2")
	cache.Del("STORAGE_ENGINE_2")
	t.Cleanup(func() {
		cache.Del("STORAGE_DEFAULT")
		cache.Del("STORAGE_ENGINE")
		cache.Del("STORAGE_DEFAULT_1")
		cache.Del("STORAGE_ENGINE_1")
		cache.Del("STORAGE_DEFAULT_2")
		cache.Del("STORAGE_ENGINE_2")
	})
	w1 := httptest.NewRecorder()
	c1, _ := gin.CreateTestContext(w1)
	ctxutil.Get(c1).TenantID = 1
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	ctxutil.Get(c2).TenantID = 2
	cache.Set(storageCacheKey(c1, "STORAGE_DEFAULT"), "qiniu", 0)
	cache.Set(storageCacheKey(c1, "STORAGE_ENGINE"), map[string]any{"domain": "https://cdn-a.example/"}, 0)
	if storageDefault(c2) == "qiniu" {
		t.Fatal("tenant 2 must not inherit tenant 1 storage default")
	}
	if storageEngine(c2, "qiniu") != nil {
		t.Fatal("tenant 2 must not inherit tenant 1 engine cache")
	}
	if got := GetFileURL(c1, "uploads/a.png"); got != "https://cdn-a.example/uploads/a.png" {
		t.Fatalf("tenant1 url=%s", got)
	}
	ClearStorageCache(c1)
	if _, ok := cache.Get("STORAGE_DEFAULT_1"); ok {
		t.Fatal("tenant cache should clear")
	}
}

func TestFetchWechatAvatarEmptyHeadimg(t *testing.T) {
	old := config.C.Project.DefaultImage
	t.Cleanup(func() { config.C.Project.DefaultImage = old })
	config.C.Project.DefaultImage = map[string]string{"user_avatar": "resource/image/common/default_avatar.png"}
	got, err := FetchWechatAvatar(nil, "openid", "")
	if err != nil || got != "resource/image/common/default_avatar.png" {
		t.Fatalf("got %q err=%v", got, err)
	}
	got, err = FetchWechatAvatar(nil, "openid", "   ")
	if err != nil || got != "resource/image/common/default_avatar.png" {
		t.Fatalf("blank %q err=%v", got, err)
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
