package cron

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
)

// ThinkFrameworkVersion is PHP `think version` for topthink/framework in this repo.
const ThinkFrameworkVersion = "v8.0.3"

func runVersion([]string) string {
	fmt.Println(ThinkFrameworkVersion)
	return ""
}

func runOptimizeSchema(args []string) string {
	if bootstrap.DB == nil {
		return ""
	}
	table := optimizeTableArg(args)
	dir := filepath.Join(runtimeRoot(), "runtime", "schema")
	if dir == filepath.Join("runtime", "schema") {
		if pub := config.C.App.PublicDir; pub != "" {
			dir = filepath.Join(filepath.Dir(pub), "runtime", "schema")
		}
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err.Error()
	}
	dbName := strings.TrimSpace(config.C.Database.Database)
	if dbName == "" {
		dbName = "likeadmin_saas"
	}
	names := []string{}
	if table != "" && table != "*" {
		names = []string{table}
	} else {
		rows, err := bootstrap.DB.Raw("SHOW TABLES").Rows()
		if err != nil {
			return err.Error()
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err.Error()
			}
			if name != "" {
				names = append(names, name)
			}
		}
	}
	for _, name := range names {
		if err := writeSchemaCache(dir, dbName, name); err != nil {
			return err.Error()
		}
	}
	return ""
}

func optimizeTableArg(args []string) string {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--table" && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-"):
			return args[i+1]
		case strings.HasPrefix(a, "--table="):
			return strings.TrimPrefix(a, "--table=")
		}
	}
	return ""
}

func writeSchemaCache(dir, dbName, table string) error {
	table = strings.Trim(table, "`")
	if table == "" {
		return nil
	}
	type col struct {
		Field string `gorm:"column:Field" json:"name"`
		Type  string `gorm:"column:Type" json:"type"`
		Null  string `gorm:"column:Null" json:"null"`
		Key   string `gorm:"column:Key" json:"key"`
	}
	var cols []col
	if err := bootstrap.DB.Raw("SHOW COLUMNS FROM `" + table + "`").Scan(&cols).Error; err != nil {
		return err
	}
	body, err := json.Marshal(cols)
	if err != nil {
		return err
	}
	name := dbName + "." + table + ".json"
	return os.WriteFile(filepath.Join(dir, name), body, 0644)
}
