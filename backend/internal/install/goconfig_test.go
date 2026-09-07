package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"likeadmin/backend/internal/config"
)

func TestWriteGoConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("app:\n  debug: true\ndatabase:\n  hostname: old\n  prefix: la_\nproject:\n  unique_identification: likeadmin\n"), 0644); err != nil {
		t.Fatal(err)
	}
	oldDB, oldPrefix, oldSalt := config.C.Database, config.C.Database.Prefix, config.C.Project.UniqueIdentification
	defer func() {
		config.C.Database = oldDB
		config.C.Database.Prefix = oldPrefix
		config.C.Project.UniqueIdentification = oldSalt
	}()
	if err := WriteGoConfig(path, "10.0.0.8", "newdb", "u", "p", 3307, "xx_", "host.test", "abcd"); err != nil {
		t.Fatal(err)
	}
	if config.C.Database.Hostname != "10.0.0.8" || config.C.Database.Prefix != "xx_" || config.C.Project.UniqueIdentification != "abcd" {
		t.Fatalf("memory %+v salt=%s", config.C.Database, config.C.Project.UniqueIdentification)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"10.0.0.8", "newdb", "xx_", "abcd", "3307"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
}
