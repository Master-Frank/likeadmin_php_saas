package generator

import (
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

// WriteModule writes generate_type=1 Vue + menu SQL + Go runtime metadata.
// PHP files are no longer written: generated tables are served by gencrud.
func WriteModule(t model.GenerateTable, files []File) error {
	c := newCtx(t, nil, time.Now())
	admin := filepath.Join(RepoRoot(), "admin", "src")
	for _, f := range files {
		dest := moduleDest(c, admin, f)
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
	return WriteGoModule(t)
}

// moduleDest returns the generate_type=1 destination, or empty to skip.
func moduleDest(c *ctx, admin string, f File) string {
	switch {
	case strings.HasSuffix(f.Name, ".php"):
		return ""
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
// but keeps already-built curd-*.zip packages so a second generate in the
// same second (or a strangler double-hit) can still be downloaded.
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
		name := e.Name()
		if strings.HasPrefix(name, "curd-") && strings.HasSuffix(name, ".zip") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
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
