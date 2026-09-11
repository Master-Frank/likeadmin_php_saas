package dbindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
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
