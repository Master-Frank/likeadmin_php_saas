package install

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

func TestWizardBlocksWhenInstalled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	lock := filepath.Join(dir, "install.lock")
	if err := os.WriteFile(lock, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	old := config.C.App.InstallLock
	t.Cleanup(func() { config.C.App.InstallLock = old })
	config.C.App.InstallLock = lock

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/install", nil)
	Wizard(c)
	if !strings.Contains(w.Body.String(), installedMsg) {
		t.Fatalf("lock should block wizard: %s", w.Body.String())
	}
}

func TestWizardServesFormWhenUnlocked(t *testing.T) {
	gin.SetMode(gin.TestMode)
	old := config.C.App.InstallLock
	t.Cleanup(func() { config.C.App.InstallLock = old })
	config.C.App.InstallLock = filepath.Join(t.TempDir(), "missing.lock")

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/install", nil)
	Wizard(c)
	body := w.Body.String()
	for _, want := range []string{"开始安装", "许可协议", "环境监测", "参数配置", "单实例", "主从库"} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in wizard: %s", want, body[:min(len(body), 400)])
		}
	}
	if strings.Contains(body, "layui") {
		t.Fatal("Go wizard must not ship the PHP layui page")
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/install/install.php", nil)
	Wizard(c2)
	if !strings.Contains(w2.Body.String(), "开始安装") {
		t.Fatalf("php alias should serve the Go wizard: %s", w2.Body.String())
	}

	eng := gin.New()
	eng.GET("/install/assets/*filepath", Asset)
	css := httptest.NewRecorder()
	eng.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/install/assets/mounted.css", nil))
	if css.Code != 200 || !strings.Contains(css.Body.String(), "--theme") {
		t.Fatalf("mounted.css: status=%d body=%s", css.Code, css.Body.String()[:min(css.Body.Len(), 200)])
	}
	for _, url := range []string{
		"/install/assets/slogn.png",
		"/install/assets/icon_mountSuccess.png",
		"/install/assets/favicon.ico",
	} {
		aw := httptest.NewRecorder()
		eng.ServeHTTP(aw, httptest.NewRequest(http.MethodGet, url, nil))
		if aw.Code != 200 || aw.Body.Len() < 100 {
			t.Fatalf("%s status=%d len=%d", url, aw.Code, aw.Body.Len())
		}
	}
	denied := httptest.NewRecorder()
	eng.ServeHTTP(denied, httptest.NewRequest(http.MethodGet, "/install/assets/../wizard.go", nil))
	if denied.Code == 200 {
		t.Fatal("asset handler must not escape wizardassets")
	}
}

func TestNewEnvInstallEntrypointsSkipPHPWizard(t *testing.T) {
	for _, rel := range []string{
		"platform/src/utils/request/index.ts",
		"tenant/src/utils/request/index.ts",
		"pc/utils/http/index.ts",
	} {
		b, err := os.ReadFile("/workspace/" + rel)
		if err != nil {
			t.Fatal(err)
		}
		txt := string(b)
		if strings.Contains(txt, "/install/install.php") {
			t.Fatalf("%s still jumps to PHP install.php", rel)
		}
		if !strings.Contains(txt, "replace('/install')") && !strings.Contains(txt, `replace("/install")`) {
			t.Fatalf("%s should jump to /install: %s", rel, txt)
		}
	}

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
		if !strings.Contains(txt, "location /install") {
			t.Fatalf("%s missing location /install", conf)
		}
		if !strings.Contains(txt, "proxy_pass") {
			t.Fatalf("%s should proxy /install to Go", conf)
		}
	}
}

func TestInstallHTTPFreshDatabase(t *testing.T) {
	if err := CheckPort("127.0.0.1", 3306); err != nil {
		t.Skip("mysql not listening")
	}
	if out, err := exec.Command("sudo", "mysql", "-e",
		"CREATE DATABASE IF NOT EXISTS `"+smokeDB+"` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"+
			"GRANT ALL PRIVILEGES ON `"+smokeDB+"`.* TO 'likeadmin'@'127.0.0.1'; FLUSH PRIVILEGES;").CombinedOutput(); err != nil {
		t.Skipf("cannot precreate smoke db: %v %s", err, out)
	}
	t.Cleanup(func() {
		_, _ = exec.Command("sudo", "mysql", "-e", "DROP DATABASE IF EXISTS `"+smokeDB+"`").CombinedOutput()
	})

	root := t.TempDir()
	pub := filepath.Join(root, "public")
	for _, sub := range []string{"uploads", "platform", "admin", "mobile"} {
		if err := os.MkdirAll(filepath.Join(pub, sub), 0755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "runtime"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(root, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	goCfg := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(goCfg, []byte("database:\n  hostname: old\n  prefix: la_\nproject:\n  unique_identification: likeadmin\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldPub, oldLock := config.C.App.PublicDir, config.C.App.InstallLock
	oldDB := config.C.Database
	oldSalt := config.C.Project.UniqueIdentification
	oldHost := config.C.Project.HTTPHost
	oldPath := config.Path
	oldBoot := bootstrap.DB
	t.Cleanup(func() {
		config.C.App.PublicDir = oldPub
		config.C.App.InstallLock = oldLock
		config.C.Database = oldDB
		config.C.Project.UniqueIdentification = oldSalt
		config.C.Project.HTTPHost = oldHost
		config.Path = oldPath
		bootstrap.DB = oldBoot
	})
	config.C.App.PublicDir = pub
	config.C.App.InstallLock = filepath.Join(cfgDir, "install.lock")
	config.Path = goCfg

	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]any{
		"host": "127.0.0.1", "port": 3306, "user": "likeadmin", "password": "root",
		"name": smokeDB, "prefix": "la_",
		"admin_user": "httpadmin", "admin_password": "likeadmin", "admin_confirm_password": "likeadmin",
		"skip_sql": 1, "env_path": filepath.Join(root, "attacker.env"),
		"go_config_path": filepath.Join(root, "attacker.yaml"),
	})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/install", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Host = "install.likeadmin.test"
	Run(c)

	var wrap response.Body
	if err := json.Unmarshal(w.Body.Bytes(), &wrap); err != nil {
		t.Fatalf("json %s: %v", w.Body.String(), err)
	}
	if wrap.Code != 1 || wrap.Msg != "安装成功" {
		t.Fatalf("install http: %+v body=%s", wrap, w.Body.String())
	}
	if bootstrap.DB == nil {
		t.Fatal("install should reconnect bootstrap.DB so tenant sharding can register")
	}
	if _, err := os.Stat(config.C.App.InstallLock); err != nil {
		t.Fatal("lock not written")
	}
	if _, err := os.Stat(filepath.Join(root, "attacker.env")); err == nil {
		t.Fatal("HTTP installer must ignore caller-controlled env_path")
	}
	if _, err := os.Stat(filepath.Join(root, "attacker.yaml")); err == nil {
		t.Fatal("HTTP installer must ignore caller-controlled go_config_path")
	}
	env, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil || !strings.Contains(string(env), smokeDB) {
		t.Fatalf("env %s err=%v", env, err)
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/install", nil)
	Wizard(c2)
	if !strings.Contains(w2.Body.String(), installedMsg) {
		t.Fatalf("wizard after install: %s", w2.Body.String())
	}
}
