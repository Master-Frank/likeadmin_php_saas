package generator

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/model"

	"gorm.io/gorm"
)

// Preview returns PHP fileInfo() items: basename + type + content.
func Preview(t model.GenerateTable, cols []model.GenerateColumn) []map[string]any {
	files := Build(t, cols)
	out := make([]map[string]any, 0, len(files))
	for _, f := range files {
		out = append(out, f.Preview())
	}
	return out
}

// Build produces the 9 PHP generator artifacts in the same order as GenerateService.
func Build(t model.GenerateTable, cols []model.GenerateColumn) []File {
	return BuildAt(t, cols, time.Now())
}

// BuildAt is Build with a fixed clock (tests / pairing).
func BuildAt(t model.GenerateTable, cols []model.GenerateColumn, now time.Time) []File {
	c := newCtx(t, cols, now)
	return []File{
		c.genController(),
		c.genLists(),
		c.genModel(),
		c.genValidate(),
		c.genLogic(),
		c.genVueAPI(),
		c.genVueIndex(),
		c.genVueEdit(),
		c.genSQL(),
	}
}

// IsZip matches PHP generate_type == 0.
func IsZip(t model.GenerateTable) bool {
	return t.GenerateType == GenerateTypeZip
}

// IsAutoMenu matches PHP SqlGenerator::isBuildMenu.
func IsAutoMenu(t model.GenerateTable) bool {
	c := newCtx(t, nil, time.Now())
	return c.menuType == GenAuto
}

// ZipRuntime packs runtime/generate/* into zipPath using PHP addFileZip
// names (generate/<rel>). Existing curd-*.zip packages are skipped.
func ZipRuntime(zipPath string) error {
	zf, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(zf)
	root := RuntimeDir()
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, "curd-") && strings.HasSuffix(name, ".zip") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		w, err := zw.Create("generate/" + rel)
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		_ = f.Close()
		return copyErr
	})
	closeErr := zw.Close()
	_ = zf.Close()
	if walkErr != nil {
		return walkErr
	}
	return closeErr
}

// WriteRuntime writes zip-mode files under runtime/generate/.
func WriteRuntime(files []File) error {
	root := RuntimeDir()
	for _, f := range files {
		path := filepath.Join(root, filepath.FromSlash(f.RelPath))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(f.Content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// WriteModule writes generate_type=1 PHP/Vue/menu SQL like PHP BaseGenerator,
// plus Go runtime metadata for gencrud. Vue/TS also land in platform/src or
// tenant/src so this repo's real admin frontends can load the generated pages.
func WriteModule(t model.GenerateTable, files []File) error {
	c := newCtx(t, nil, time.Now())
	admin := filepath.Join(RepoRoot(), "admin", "src")
	for _, f := range files {
		for _, dest := range moduleDests(c, admin, f) {
			if dest == "" {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(dest, []byte(f.Content), 0644); err != nil {
				return err
			}
		}
	}
	return WriteGoModule(t)
}

// frontendSrc is the real Vue app for a generator module. PHP still writes
// admin/src (pair.sh cleans those paths); this repo's UIs live in platform/
// and tenant/.
func frontendSrc(module string) string {
	switch goModuleApp(module) {
	case "platformapi":
		return filepath.Join(RepoRoot(), "platform", "src")
	case "tenantapi":
		return filepath.Join(RepoRoot(), "tenant", "src")
	default:
		return ""
	}
}

// moduleDests returns generate_type=1 destinations. Vue/TS are written to
// admin/src (PHP pairing) and the matching platform/ or tenant/ frontend.
func moduleDests(c *ctx, admin string, f File) []string {
	dest := moduleDest(c, admin, f)
	if dest == "" {
		return nil
	}
	out := []string{dest}
	extra := frontendSrc(c.module)
	if extra == "" || extra == admin {
		return out
	}
	switch {
	case strings.HasSuffix(f.Name, ".ts"), f.Name == "index.vue", f.Name == "edit.vue":
		if d := moduleDest(c, extra, f); d != "" && d != dest {
			out = append(out, d)
		}
	}
	return out
}

// moduleDest returns the generate_type=1 destination, or empty to skip.
func moduleDest(c *ctx, admin string, f File) string {
	app := filepath.Join(ServerRoot(), "app")
	switch {
	case strings.HasSuffix(f.Name, "Controller.php"):
		return filepath.Join(joinClassDir(filepath.Join(app, c.module, "controller"), c.classDir), f.Name)
	case strings.HasSuffix(f.Name, "Lists.php"):
		return filepath.Join(joinClassDir(filepath.Join(app, c.module, "lists"), c.classDir), f.Name)
	case strings.HasSuffix(f.Name, "Logic.php"):
		return filepath.Join(joinClassDir(filepath.Join(app, c.module, "logic"), c.classDir), f.Name)
	case strings.HasSuffix(f.Name, "Validate.php"):
		return filepath.Join(joinClassDir(filepath.Join(app, c.module, "validate"), c.classDir), f.Name)
	case strings.HasSuffix(f.Name, ".php"):
		return filepath.Join(joinClassDir(filepath.Join(app, "common", "model"), c.classDir), f.Name)
	case strings.HasSuffix(f.Name, ".ts"):
		return filepath.Join(admin, "api", f.Name)
	case f.Name == "index.vue":
		return filepath.Join(admin, "views", c.lowerTable(), f.Name)
	case f.Name == "edit.vue":
		return filepath.Join(admin, "views", c.tableName, f.Name)
	case f.Name == "menu.sql":
		return filepath.Join(RuntimeDir(), "sql", f.Name)
	default:
		return ""
	}
}

// WriteGoModule writes generated Go metadata under backend/internal/generated.
func WriteGoModule(t model.GenerateTable) error {
	root := filepath.Join(RepoRoot(), "backend", "internal", "generated")
	for _, f := range BuildGo(t, nil) {
		dest := filepath.Join(root, f.Name)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(f.Content), 0644); err != nil {
			return err
		}
	}
	return nil
}

// ClearRuntime empties runtime/generate like PHP delGenerateDirContent,
// including prior curd-*.zip packages.
func ClearRuntime() error {
	root := RuntimeDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(root, 0755)
		}
		return err
	}
	for _, e := range entries {
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err != nil {
			return err
		}
	}
	return os.MkdirAll(root, 0755)
}

// ApplyMenuSQL executes generated menu SQL on the same connection (LAST_INSERT_ID / @pid).
func ApplyMenuSQL(db *gorm.DB, sqlText string) error {
	if db == nil || strings.TrimSpace(sqlText) == "" {
		return nil
	}
	return db.Connection(func(tx *gorm.DB) error {
		for _, stmt := range strings.Split(sqlText, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
