package router

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

func TestServeSPAPrefersRealAssets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	assets := filepath.Join(dir, "platform", "assets")
	if err := os.MkdirAll(assets, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "platform", "index.html"), []byte("<html>spa-index</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index-abc.js"), []byte("console.log('ok')"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(assets, "index-abc.css"), []byte("body{color:red}"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.PublicDir
	t.Cleanup(func() { config.C.App.PublicDir = old })
	config.C.App.PublicDir = dir

	hit := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		serveSPA("platform")(c)
		return w
	}

	js := hit("/platform/assets/index-abc.js")
	if !strings.Contains(js.Body.String(), "console.log") {
		t.Fatalf("js body %q", js.Body.String())
	}
	ct := js.Header().Get("Content-Type")
	if !strings.Contains(ct, "javascript") && !strings.Contains(ct, "ecmascript") {
		t.Fatalf("js content-type %q", ct)
	}

	css := hit("/platform/assets/index-abc.css")
	if !strings.Contains(css.Body.String(), "body{color:red}") {
		t.Fatalf("css body %q", css.Body.String())
	}
	if !strings.Contains(css.Header().Get("Content-Type"), "css") {
		t.Fatalf("css content-type %q", css.Header().Get("Content-Type"))
	}

	page := hit("/platform/login")
	if !strings.Contains(page.Body.String(), "spa-index") {
		t.Fatalf("missing spa fallback %q", page.Body.String())
	}
	if !strings.Contains(page.Header().Get("Content-Type"), "html") {
		t.Fatalf("html content-type %q", page.Header().Get("Content-Type"))
	}
}

func TestServeSPARejectsDotDot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "platform"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "platform", "index.html"), []byte("index"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.PublicDir
	t.Cleanup(func() { config.C.App.PublicDir = old })
	config.C.App.PublicDir = dir

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/platform/../secret.txt", nil)
	serveSPA("platform")(c)
	if strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("escaped public_dir: %q", w.Body.String())
	}
}

func TestRedirectShopToPC(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hit := func(path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, path, nil)
		redirectShopToPC(c)
		return w
	}

	news := hit("/pages/news/news")
	if news.Code != http.StatusFound {
		t.Fatalf("status %d", news.Code)
	}
	if loc := news.Header().Get("Location"); loc != "/pc/information" {
		t.Fatalf("location %q", loc)
	}

	q := hit("/pages/news_detail/news_detail?id=3")
	if loc := q.Header().Get("Location"); loc != "/pc/information/detail/3" {
		t.Fatalf("query location %q", loc)
	}

	pkg := hit("/packages/pages/user_wallet/user_wallet")
	if loc := pkg.Header().Get("Location"); loc != "/pc/" {
		t.Fatalf("package location %q", loc)
	}

	h5 := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(h5)
	c.Request = httptest.NewRequest(http.MethodGet, "/pages/news_detail/news_detail?id=3", nil)
	c.Request.Header.Set("Referer", "http://127.0.0.1:8080/mobile/")
	redirectShopToPC(c)
	if loc := h5.Header().Get("Location"); loc != "/mobile/pages/news_detail/news_detail?id=3" {
		t.Fatalf("mobile referer %q", loc)
	}
}

func TestRedirectToPCApp(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/information/detail/6", nil)
	redirectToPCApp(c)
	if w.Code != http.StatusFound {
		t.Fatalf("status %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/pc/information/detail/6" {
		t.Fatalf("location %q", loc)
	}
}

func TestSiteRootRedirectsPlatformHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "platform"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "platform", "index.html"), []byte("platform-spa"), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPub, oldHost := config.C.App.PublicDir, config.C.Project.HTTPHost
	t.Cleanup(func() {
		config.C.App.PublicDir = oldPub
		config.C.Project.HTTPHost = oldHost
	})
	config.C.App.PublicDir = dir
	config.C.Project.HTTPHost = "demo.gosaas.cn"

	hit := func(host string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Request.Host = host
		siteRoot(c)
		return w
	}

	plat := hit("demo.gosaas.cn")
	if plat.Code != http.StatusFound || plat.Header().Get("Location") != "/platform/" {
		t.Fatalf("platform host: %d %s", plat.Code, plat.Header().Get("Location"))
	}

	tenant := hit("shop.gosaas.cn")
	if tenant.Code != http.StatusOK || !strings.Contains(tenant.Body.String(), "platform-spa") {
		t.Fatalf("tenant host should serve platform index: %d %s", tenant.Code, tenant.Body.String())
	}

	config.C.Project.HTTPHost = ""
	empty := hit("anything.example")
	if empty.Code != http.StatusFound || empty.Header().Get("Location") != "/platform/" {
		t.Fatalf("empty http_host: %d %s", empty.Code, empty.Header().Get("Location"))
	}
}

func TestNginxRootProxiesToGo(t *testing.T) {
	for _, conf := range []string{
		"/workspace/backend/deploy/nginx.production.conf",
		"/workspace/backend/deploy/nginx.prod.conf",
		"/workspace/backend/deploy/nginx-strangler.conf",
	} {
		b, err := os.ReadFile(conf)
		if err != nil {
			t.Fatal(err)
		}
		txt := string(b)
		if !strings.Contains(txt, "location = /") {
			t.Fatalf("%s missing location = / so nginx would serve public/index.html", conf)
		}
	}
}
