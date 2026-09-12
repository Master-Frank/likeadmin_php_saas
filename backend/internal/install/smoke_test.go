package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/util"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const smokeDB = "likeadmin_install_smoke"

func TestApplyRefusesProtectedPairingDB(t *testing.T) {
	_, err := Apply(Options{
		Host: "127.0.0.1", Port: 3306, User: "likeadmin", Password: "root",
		Name: "localhost_likeadmin", Prefix: "la_",
		AdminUser: "admin", AdminPassword: "likeadmin",
	})
	if err == nil || !strings.Contains(err.Error(), "受保护数据库") {
		t.Fatalf("must refuse pairing db: %v", err)
	}
}

func TestApplyFreshDatabaseLikePHP(t *testing.T) {
	if err := CheckPort("127.0.0.1", 3306); err != nil {
		t.Skip("mysql not listening")
	}
	// likeadmin 只有业务库权限，空库由 socket root 预建，避免授予全局 CREATE/DROP。
	if out, err := exec.Command("sudo", "mysql", "-e",
		"CREATE DATABASE IF NOT EXISTS `"+smokeDB+"` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"+
			"GRANT ALL PRIVILEGES ON `"+smokeDB+"`.* TO 'likeadmin'@'127.0.0.1'; FLUSH PRIVILEGES;").CombinedOutput(); err != nil {
		t.Skipf("cannot precreate smoke db: %v %s", err, out)
	}
	t.Cleanup(func() {
		_, _ = exec.Command("sudo", "mysql", "-e", "DROP DATABASE IF EXISTS `"+smokeDB+"`").CombinedOutput()
	})
	root, err := gorm.Open(mysql.Open("likeadmin:root@tcp(127.0.0.1:3306)/"+smokeDB+"?charset=utf8mb4&parseTime=false&loc=Local"), &gorm.Config{})
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() {
		if sqlDB, e := root.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "public"), 0755); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(dir, "config", "install.lock")
	envPath := filepath.Join(dir, ".env")
	goCfg := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(goCfg, []byte("database:\n  hostname: old\n  prefix: la_\nproject:\n  unique_identification: likeadmin\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	oldDB := config.C.Database
	oldSalt := config.C.Project.UniqueIdentification
	oldHost := config.C.Project.HTTPHost
	t.Cleanup(func() {
		config.C.Database = oldDB
		config.C.Project.UniqueIdentification = oldSalt
		config.C.Project.HTTPHost = oldHost
	})

	ts := time.Date(2026, 9, 8, 4, 0, 0, 0, time.UTC).Unix()
	res, err := Apply(Options{
		Host: "127.0.0.1", Port: 3306, User: "likeadmin", Password: "root",
		Name: smokeDB, Prefix: "xx_",
		AdminUser: "smokeadmin", AdminPassword: "likeadmin",
		PublicDir: filepath.Join(dir, "public"),
		LockPath:  lock, EnvPath: envPath, GoConfigPath: goCfg,
		HTTPHost: "install.likeadmin.test", Now: ts,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Imported == 0 {
		t.Fatal("imported 0 statements")
	}
	if st, err := os.Stat(lock); err != nil || st.Size() != 0 {
		t.Fatalf("lock should be empty touch file: %v size=%v", err, st)
	}
	envBody, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatal(err)
	}
	env := string(envBody)
	for _, want := range []string{
		`DATABASE = "` + smokeDB + `"`,
		`PREFIX = "xx_"`,
		`USERNAME = "likeadmin"`,
		`HOSTNAME = "127.0.0.1"`,
		`UNIQUE_IDENTIFICATION = "` + res.Salt + `"`,
		`HTTP_HOST = "install.likeadmin.test"`,
	} {
		if !strings.Contains(env, want) {
			t.Fatalf("env missing %s\n%s", want, env)
		}
	}
	goBody, err := os.ReadFile(goCfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goBody), smokeDB) || !strings.Contains(string(goBody), "xx_") {
		t.Fatalf("go config not updated: %s", goBody)
	}

	wantCreates := countLikeSQLCreates(t)
	db, err := gorm.Open(mysql.Open(fmt.Sprintf("likeadmin:root@tcp(127.0.0.1:3306)/%s?charset=utf8mb4&parseTime=false&loc=Local", smokeDB)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	if err := db.Raw("SHOW TABLES LIKE 'xx_%'").Scan(&tables).Error; err != nil {
		t.Fatal(err)
	}
	if len(tables) != wantCreates {
		t.Fatalf("tables %d want %d: %v", len(tables), wantCreates, tables)
	}

	var admin struct {
		Account  string
		Password string
		Root     int
	}
	if err := db.Raw("SELECT account, password, root FROM `xx_admin` WHERE id=1").Scan(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if admin.Account != "smokeadmin" || admin.Root != 1 {
		t.Fatalf("admin %+v", admin)
	}
	wantPwd := util.CreatePassword("likeadmin", AccountSalt(ts, "smokeadmin"))
	if admin.Password != wantPwd {
		t.Fatalf("password %s want %s salt=%s", admin.Password, wantPwd, res.Salt)
	}
	var dept int
	if err := db.Raw("SELECT COUNT(*) FROM `xx_admin_dept` WHERE admin_id=1 AND dept_id=1").Scan(&dept).Error; err != nil || dept != 1 {
		t.Fatalf("admin_dept %d err=%v", dept, err)
	}

	_, err = Apply(Options{
		Host: "127.0.0.1", Port: 3306, User: "likeadmin", Password: "root",
		Name: smokeDB, Prefix: "xx_",
		AdminUser: "smokeadmin", AdminPassword: "likeadmin",
		PublicDir: filepath.Join(dir, "public"),
		LockPath:  filepath.Join(dir, "again.lock"),
		EnvPath:   filepath.Join(dir, "again.env"),
	})
	if err == nil || !strings.Contains(err.Error(), "数据表已存在") {
		t.Fatalf("second install without clear_db: %v", err)
	}

	if out, err := exec.Command("sudo", "mysql", "-e",
		"GRANT CREATE, DROP ON *.* TO 'likeadmin'@'127.0.0.1'; FLUSH PRIVILEGES;").CombinedOutput(); err != nil {
		t.Fatalf("grant create/drop: %v %s", err, out)
	}
	t.Cleanup(func() {
		_, _ = exec.Command("sudo", "mysql", "-e",
			"REVOKE CREATE, DROP ON *.* FROM 'likeadmin'@'127.0.0.1'; FLUSH PRIVILEGES;").CombinedOutput()
	})
	res2, err := Apply(Options{
		Host: "127.0.0.1", Port: 3306, User: "likeadmin", Password: "root",
		Name: smokeDB, Prefix: "xx_", ClearDB: true,
		AdminUser: "again", AdminPassword: "likeadmin",
		PublicDir: filepath.Join(dir, "public"),
		LockPath:  filepath.Join(dir, "clear.lock"),
		EnvPath:   filepath.Join(dir, "clear.env"),
		Now:       ts + 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Imported == 0 {
		t.Fatal("clear_db imported 0")
	}
	db2, err := gorm.Open(mysql.Open(fmt.Sprintf("likeadmin:root@tcp(127.0.0.1:3306)/%s?charset=utf8mb4&parseTime=false&loc=Local", smokeDB)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	var again string
	if err := db2.Raw("SELECT account FROM `xx_admin` WHERE id=1").Scan(&again).Error; err != nil || again != "again" {
		t.Fatalf("clear_db admin %q err=%v", again, err)
	}
}

func countLikeSQLCreates(t *testing.T) int {
	t.Helper()
	raw, err := ReadLikeSQL("")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, stmt := range SplitSQL(string(raw)) {
		u := strings.ToUpper(strings.TrimSpace(stmt))
		if strings.HasPrefix(u, "CREATE TABLE") {
			n++
		}
	}
	if n == 0 {
		t.Fatal("like.sql has no CREATE TABLE")
	}
	return n
}
