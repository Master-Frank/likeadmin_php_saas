package upgrade

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestartCommandRequiredAndNoShell(t *testing.T) {
	t.Setenv(restartCommandEnv, "")
	if requireRestartCommand() == nil {
		t.Fatal("Go upgrade must require a restart command")
	}
	t.Setenv(restartCommandEnv, `["/bin/true"]`)
	if err := requireRestartCommand(); err != nil {
		t.Fatal(err)
	}
	if err := runRestartCommand(); err != nil {
		t.Fatal(err)
	}
	t.Setenv(restartCommandEnv, `["true"]`)
	if runRestartCommand() == nil {
		t.Fatal("relative restart programs must be rejected")
	}
}

func TestStageGoUpgradeBuildsDeployableBinaries(t *testing.T) {
	live := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		path := filepath.Join(live, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.test/upgrade\n\ngo 1.22\n")
	write("cmd/api/main.go", "package main\nfunc main(){}\n")
	write("cmd/crontab/main.go", "package main\nfunc main(){}\n")

	extract := t.TempDir()
	patch := filepath.Join(extract, "project", "backend", "internal", "version")
	if err := os.MkdirAll(patch, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(patch, "version.go"), []byte("package version\nconst V = 2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	staged, err := stageGoUpgrade(extract, live)
	if err != nil {
		t.Fatal(err)
	}
	if staged == nil {
		t.Fatal("backend patch should stage Go binaries")
	}
	defer staged.cleanup()
	if err := staged.install(live); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"api", "crontab"} {
		if st, err := os.Stat(filepath.Join(live, "bin", name)); err != nil || st.Mode()&0o111 == 0 {
			t.Fatalf("%s not installed executable: %v", name, err)
		}
	}
}
