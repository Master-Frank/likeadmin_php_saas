package generator

import (
	"embed"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/config"
)

//go:embed stub
var stubFS embed.FS

func findServerRoot() string {
	if pub := config.C.App.PublicDir; pub != "" {
		root := filepath.Dir(pub)
		if st, err := os.Stat(root); err == nil && st.IsDir() {
			return root
		}
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

func findBackendRoot() string {
	wd, _ := os.Getwd()
	for d := wd; d != "" && d != "/"; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "internal", "generator")); err == nil {
			if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
				return d
			}
		}
		if _, err := os.Stat(filepath.Join(d, "backend", "internal", "generator")); err == nil {
			return filepath.Join(d, "backend")
		}
		if d == filepath.Dir(d) {
			break
		}
	}
	if pub := config.C.App.PublicDir; pub != "" {
		repo := filepath.Dir(filepath.Dir(pub))
		if st, err := os.Stat(filepath.Join(repo, "backend")); err == nil && st.IsDir() {
			return filepath.Join(repo, "backend")
		}
	}
	return "backend"
}

// ServerRoot is the PHP project root when it still exists (pair / dual-stack).
func ServerRoot() string {
	return findServerRoot()
}

// RepoRoot is the parent of the PHP server root (where platform/ and tenant/ live).
func RepoRoot() string {
	backend := findBackendRoot()
	if filepath.Base(backend) == "backend" {
		return filepath.Dir(backend)
	}
	return filepath.Dir(ServerRoot())
}

// StubDir is the embedded stub tree. Kept for test error messages.
func StubDir() string {
	return "embed:stub"
}

// RuntimeDir is backend/runtime/generate — independent of server/.
func RuntimeDir() string {
	return filepath.Join(findBackendRoot(), "runtime", "generate")
}

func stubName(name string) string {
	return "stub/" + strings.TrimSuffix(filepath.ToSlash(name), ".stub") + ".stub"
}

func readStub(name string) string {
	b, err := stubFS.ReadFile(stubName(name))
	if err != nil {
		return ""
	}
	return string(b)
}

func stubExists(name string) bool {
	f, err := stubFS.Open(stubName(name))
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
