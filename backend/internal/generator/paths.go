package generator

import (
	"os"
	"path/filepath"

	"likeadmin/backend/internal/config"
)

func findServerRoot() string {
	if pub := config.C.App.PublicDir; pub != "" {
		return filepath.Dir(pub)
	}
	wd, _ := os.Getwd()
	for d := wd; d != "" && d != "/"; d = filepath.Dir(d) {
		if st, err := os.Stat(filepath.Join(d, "server", "app")); err == nil && st.IsDir() {
			return filepath.Join(d, "server")
		}
		if st, err := os.Stat(filepath.Join(d, "app", "common")); err == nil && st.IsDir() {
			return d
		}
	}
	return "server"
}

// ServerRoot is the PHP project root (the directory that contains app/ and runtime/).
func ServerRoot() string {
	return findServerRoot()
}

// RepoRoot is the parent of the PHP server root (where admin/ would live).
func RepoRoot() string {
	return filepath.Dir(ServerRoot())
}

// StubDir is PHP stub/ used by the generators.
func StubDir() string {
	return filepath.Join(ServerRoot(), "app", "common", "service", "generator", "stub")
}

// RuntimeDir is PHP runtime/generate/.
func RuntimeDir() string {
	return filepath.Join(ServerRoot(), "runtime", "generate")
}

func stubPath(name string) string {
	return filepath.Join(StubDir(), name+".stub")
}

func readStub(name string) string {
	b, err := os.ReadFile(stubPath(name))
	if err != nil {
		return ""
	}
	return string(b)
}

func stubExists(name string) bool {
	_, err := os.Stat(stubPath(name))
	return err == nil
}

func joinClassDir(base, classDir string) string {
	if classDir == "" {
		return base
	}
	return filepath.Join(base, classDir)
}
