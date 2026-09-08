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
	if !strings.Contains(w.Body.String(), "开始安装") {
		t.Fatalf("expected form: %s", w.Body.String())
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest(http.MethodGet, "/install/install.php", nil)
	Wizard(c2)
	if !strings.Contains(w2.Body.String(), "开始安装") {
		t.Fatalf("php alias should serve the Go wizard: %s", w2.Body.String())
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
	pub := "/workspace/server/public"
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
	oldBoot := bootstrap.DB
	t.Cleanup(func() {
		config.C.App.PublicDir = oldPub
		config.C.App.InstallLock = oldLock
		config.C.Database = oldDB
		config.C.Project.UniqueIdentification = oldSalt
		config.C.Project.HTTPHost = oldHost
		bootstrap.DB = oldBoot
	})
	config.C.App.PublicDir = pub
	config.C.App.InstallLock = filepath.Join(cfgDir, "install.lock")

	gin.SetMode(gin.TestMode)
	body, _ := json.Marshal(map[string]any{
		"host": "127.0.0.1", "port": 3306, "user": "likeadmin", "password": "root",
		"name": smokeDB, "prefix": "la_",
		"admin_user": "httpadmin", "admin_password": "likeadmin", "admin_confirm_password": "likeadmin",
		"go_config_path": goCfg, "env_path": filepath.Join(root, ".env"),
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
	if _, err := os.Stat(config.C.App.InstallLock); err != nil {
		t.Fatal("lock not written")
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
