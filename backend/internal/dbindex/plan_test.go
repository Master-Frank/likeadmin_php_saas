package dbindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"

	"gorm.io/gorm"
)

func TestPlanNilDB(t *testing.T) {
	items := Plan(nil)
	if len(items) == 0 {
		t.Fatal("plan should list candidate indexes without a DB")
	}
	if Missing(nil) != nil {
		t.Fatal("missing on nil DB")
	}
	txt := FormatPlan(items)
	if !strings.Contains(txt, "idx_sn_delete_time") || !strings.Contains(txt, "skipped") {
		t.Fatalf("%s", txt)
	}
}

func TestWriteStatusNilOK(t *testing.T) {
	dir := t.TempDir()
	old := config.C.App.PublicDir
	config.C.App.PublicDir = filepath.Join(dir, "public")
	t.Cleanup(func() { config.C.App.PublicDir = old })
	WriteStatus(Plan(nil), nil)
	if _, err := os.Stat(StatusPath()); err != nil {
		t.Fatal(err)
	}
}

func TestMysqlOnlineDDLVersion(t *testing.T) {
	if !mysqlOnlineDDLVersion("8.0.36") || !mysqlOnlineDDLVersion("5.7.8") {
		t.Fatal("supported")
	}
	if mysqlOnlineDDLVersion("5.6.10") || mysqlOnlineDDLVersion("5.7.7") {
		t.Fatal("old")
	}
	if !mysqlOnlineDDLVersion("8.0.36-0ubuntu0.22.04.1") {
		t.Fatal("suffix")
	}
}

func TestAddIndexSQL(t *testing.T) {
	got := addIndexSQL(true, "la_user", "idx_x", "`id`")
	if !strings.Contains(got, "ALGORITHM=INPLACE") || !strings.Contains(got, "LOCK=NONE") {
		t.Fatalf("%s", got)
	}
	if addIndexSQL(false, "la_user", "idx_x", "`id`") != "CREATE INDEX `idx_x` ON `la_user` (`id`)" {
		t.Fatal(addIndexSQL(false, "la_user", "idx_x", "`id`"))
	}
}

func TestExplainQueriesAndNilDB(t *testing.T) {
	if len(ExplainQueries()) < 6 {
		t.Fatal("missing shapes")
	}
	if RunExplain(nil) != "database unavailable" {
		t.Fatal(RunExplain(nil))
	}
}

func TestWithIndexLockNonMySQLRunsFunction(t *testing.T) {
	ran := false
	if !withIndexLock(&gorm.DB{}, func() { ran = true }) || !ran {
		t.Fatal("non-MySQL test DB should execute without advisory lock")
	}
}
