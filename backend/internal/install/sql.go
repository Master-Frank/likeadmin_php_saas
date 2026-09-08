package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/sqlassets"

	"gorm.io/gorm"
)

// SplitSQL splits a MySQL dump the way PHP install createTable does.
func SplitSQL(content string) []string {
	content = strings.ReplaceAll(content, ";\r\n", ";\n")
	parts := strings.Split(content, ";\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.HasPrefix(p, "--") {
			continue
		}
		out = append(out, p)
	}
	return out
}

func rewritePrefix(stmt, prefix string) string {
	if prefix == "" || prefix == "la_" {
		return stmt
	}
	return strings.ReplaceAll(stmt, "`la_", "`"+prefix)
}

// qualifyInstallSQL mirrors PHP installModel::createTable:
// `la_` → {dbName}.`la_` then `la_` → `{prefix}.
func qualifyInstallSQL(stmt, dbName, prefix string) string {
	if dbName != "" {
		stmt = strings.ReplaceAll(stmt, "`la_", dbName+".`la_")
	}
	return rewritePrefix(stmt, prefix)
}

func ImportSQL(db *gorm.DB, content, prefix string, dbName ...string) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("数据库未连接")
	}
	name := ""
	if len(dbName) > 0 {
		name = dbName[0]
	}
	n := 0
	for _, stmt := range SplitSQL(content) {
		stmt = qualifyInstallSQL(stmt, name, prefix)
		if err := db.Exec(stmt).Error; err != nil {
			return n, fmt.Errorf("执行 SQL 失败: %w", err)
		}
		n++
	}
	return n, nil
}

func FindLikeSQL(publicDir string) string {
	cands := []string{
		filepath.Join(publicDir, "install", "db", "like.sql"),
		filepath.Join(publicDir, "..", "public", "install", "db", "like.sql"),
	}
	for _, p := range cands {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ReadLikeSQL prefers a customized dump under public_dir, then the embed.
func ReadLikeSQL(publicDir string) ([]byte, error) {
	if p := FindLikeSQL(publicDir); p != "" {
		return os.ReadFile(p)
	}
	if sqlassets.LikeSQL != "" {
		return []byte(sqlassets.LikeSQL), nil
	}
	return nil, fmt.Errorf("创建表格失败")
}
