package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func ImportSQL(db *gorm.DB, content, prefix string) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("数据库未连接")
	}
	n := 0
	for _, stmt := range SplitSQL(content) {
		stmt = rewritePrefix(stmt, prefix)
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

func WriteEnv(path string, host, dbName, user, pass string, port int, prefix, httpHost string) error {
	if prefix == "" {
		prefix = "la_"
	}
	if port == 0 {
		port = 3306
	}
	content := fmt.Sprintf("APP_DEBUG = true\n\n[APP]\nDEFAULT_TIMEZONE = Asia/Shanghai\n\n[DATABASE]\nTYPE = mysql\nHOSTNAME = \"%s\"\nDATABASE = \"%s\"\nUSERNAME = \"%s\"\nPASSWORD = \"%s\"\nHOSTPORT = \"%d\"\nCHARSET = utf8mb4\nPREFIX = \"%s\"\n\n[PROJECT]\nUNIQUE_IDENTIFICATION = likeadmin\nHTTP_HOST = \"%s\"\n",
		host, dbName, user, pass, port, prefix, httpHost)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}
