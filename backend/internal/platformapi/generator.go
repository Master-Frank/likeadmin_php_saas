package platformapi

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func GeneratorDataTable(c *gin.Context) {
	q := lists.Parse(c)
	type row struct {
		Name       string `gorm:"column:Name" json:"Name"`
		Comment    string `gorm:"column:Comment" json:"Comment"`
		Engine     string `gorm:"column:Engine" json:"Engine"`
		Rows       int64  `gorm:"column:Rows" json:"Rows"`
		Collation  string `gorm:"column:Collation" json:"Collation"`
		CreateTime string `gorm:"column:Create_time" json:"Create_time"`
		UpdateTime string `gorm:"column:Update_time" json:"Update_time"`
	}
	sql := "SELECT TABLE_NAME AS `Name`, TABLE_COMMENT AS `Comment`, ENGINE AS `Engine`, TABLE_ROWS AS `Rows`, TABLE_COLLATION AS `Collation`, CREATE_TIME AS `Create_time`, UPDATE_TIME AS `Update_time` FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME LIKE ?"
	like := config.Prefix() + "%"
	if kw := lists.Param(q, "table_name"); kw != "" {
		like = "%" + kw + "%"
	}
	var rows []row
	bootstrap.DB.Raw(sql, like).Scan(&rows)
	count := int64(len(rows))
	start := q.Offset
	end := q.Offset + q.PageSize
	if start > len(rows) {
		start = len(rows)
	}
	if end > len(rows) {
		end = len(rows)
	}
	response.Lists(c, rows[start:end], count, q.PageNo, q.PageSize, nil)
}

func GeneratorGenerateTable(c *gin.Context) {
	q := lists.Parse(c)
	db := bootstrap.DB.Model(&model.GenerateTable{})
	if n := lists.Param(q, "table_name"); n != "" {
		db = db.Where("table_name LIKE ?", "%"+n+"%")
	}
	var count int64
	db.Count(&count)
	var rows []model.GenerateTable
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "table_name": r.Name, "table_comment": r.TableComment,
			"author": r.Author, "remark": r.Remark, "create_time": util.FormatDateTime(r.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func GeneratorSelectTable(c *gin.Context) {
	tables := httpx.Any(c, "table")
	arr, _ := tables.([]any)
	if arr == nil {
		if s := httpx.Str(c, "table"); s != "" {
			arr = []any{s}
		}
	}
	adminID := ctxutil.Get(c).AdminID
	now := util.NowUnix()
	for _, item := range arr {
		name := ""
		comment := ""
		if m, ok := item.(map[string]any); ok {
			name = util.ToString(m["name"])
			if name == "" {
				name = util.ToString(m["table_name"])
			}
			comment = util.ToString(m["comment"])
			if comment == "" {
				comment = util.ToString(m["table_comment"])
			}
		} else {
			name = util.ToString(item)
		}
		if name == "" {
			continue
		}
		gt := model.GenerateTable{
			Name: name, TableComment: comment, Author: "likeadmin",
			ModuleName: "admin", ClassDir: strings.TrimPrefix(name, config.Prefix()),
			AdminID: adminID, CreateTime: now,
		}
		if err := bootstrap.DB.Create(&gt).Error; err != nil {
			response.Fail(c, err.Error())
			return
		}
		syncColumns(gt.ID, name)
	}
	response.Success(c, "操作成功", nil)
}

func GeneratorDetail(c *gin.Context) {
	var t model.GenerateTable
	if bootstrap.DB.First(&t, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "记录不存在")
		return
	}
	var cols []model.GenerateColumn
	bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
	response.Data(c, gin.H{"base": t, "column": cols})
}

func GeneratorSyncColumn(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var t model.GenerateTable
	if bootstrap.DB.First(&t, id).Error != nil {
		response.Fail(c, "记录不存在")
		return
	}
	bootstrap.DB.Where("table_id = ?", id).Delete(&model.GenerateColumn{})
	syncColumns(id, t.Name)
	response.Success(c, "同步成功", nil)
}

func GeneratorDelete(c *gin.Context) {
	ids := httpx.Uints(c, "id")
	if len(ids) == 0 {
		ids = httpx.Uints(c, "ids")
	}
	if id := httpx.Uint(c, "id"); id > 0 && len(ids) == 0 {
		ids = []uint{id}
	}
	if len(ids) > 0 {
		bootstrap.DB.Where("id IN ?", ids).Delete(&model.GenerateTable{})
		bootstrap.DB.Where("table_id IN ?", ids).Delete(&model.GenerateColumn{})
	}
	response.Success(c, "删除成功", nil)
}

func GeneratorEdit(c *gin.Context) {
	id := httpx.Uint(c, "id")
	bootstrap.DB.Model(&model.GenerateTable{}).Where("id = ?", id).Updates(map[string]any{
		"table_comment": httpx.Str(c, "table_comment"), "author": httpx.Str(c, "author"),
		"remark": httpx.Str(c, "remark"), "module_name": httpx.Str(c, "module_name"),
		"class_dir": httpx.Str(c, "class_dir"), "class_comment": httpx.Str(c, "class_comment"),
		"generate_type": httpx.Int(c, "generate_type"), "update_time": util.NowUnix(),
	})
	response.Success(c, "修改成功", nil)
}

func GeneratorPreview(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var t model.GenerateTable
	if bootstrap.DB.First(&t, id).Error != nil {
		response.Fail(c, "记录不存在")
		return
	}
	var cols []model.GenerateColumn
	bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
	goCode, vueCode := renderGoVue(t, cols)
	response.Success(c, "", []map[string]any{
		{"name": t.ClassDir + ".go", "type": "go", "content": goCode},
		{"name": t.ClassDir + "/lists.vue", "type": "vue", "content": vueCode},
	})
}

func GeneratorGenerate(c *gin.Context) {
	ids := httpx.Uints(c, "id")
	if len(ids) == 0 {
		ids = httpx.Uints(c, "ids")
	}
	if id := httpx.Uint(c, "id"); id > 0 && len(ids) == 0 {
		ids = []uint{id}
	}
	root := filepath.Join(filepath.Dir(config.C.App.PublicDir), "..", "backend", "internal", "generated")
	if wd, err := os.Getwd(); err == nil {
		if strings.HasSuffix(wd, "backend") {
			root = filepath.Join(wd, "internal", "generated")
		}
	}
	_ = os.MkdirAll(root, 0755)
	for _, id := range ids {
		var t model.GenerateTable
		if bootstrap.DB.First(&t, id).Error != nil {
			continue
		}
		var cols []model.GenerateColumn
		bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
		goCode, vueCode := renderGoVue(t, cols)
		dir := filepath.Join(root, t.ClassDir)
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, t.ClassDir+".go"), []byte(goCode), 0644)
		_ = os.WriteFile(filepath.Join(dir, "lists.vue"), []byte(vueCode), 0644)
	}
	response.Success(c, "生成成功", nil)
}

func GeneratorGetModels(c *gin.Context) {
	response.Data(c, []string{
		"Admin", "User", "Tenant", "Article", "ArticleCate", "File", "Dept", "Jobs",
	})
}

func syncColumns(tableID uint, tableName string) {
	type col struct {
		ColumnName    string `gorm:"column:COLUMN_NAME"`
		ColumnComment string `gorm:"column:COLUMN_COMMENT"`
		ColumnType    string `gorm:"column:COLUMN_TYPE"`
		ColumnKey     string `gorm:"column:COLUMN_KEY"`
		IsNullable    string `gorm:"column:IS_NULLABLE"`
	}
	var cols []col
	bootstrap.DB.Raw("SELECT COLUMN_NAME, COLUMN_COMMENT, COLUMN_TYPE, COLUMN_KEY, IS_NULLABLE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION", tableName).Scan(&cols)
	now := util.NowUnix()
	for _, col := range cols {
		pk, req := 0, 0
		if col.ColumnKey == "PRI" {
			pk = 1
		}
		if col.IsNullable == "NO" && pk == 0 {
			req = 1
		}
		bootstrap.DB.Create(&model.GenerateColumn{
			TableID: tableID, ColumnName: col.ColumnName, ColumnComment: col.ColumnComment,
			ColumnType: col.ColumnType, IsPk: pk, IsRequired: req, IsInsert: 1, IsUpdate: 1,
			IsLists: 1, QueryType: "=", ViewType: "input", CreateTime: now,
		})
	}
}

func renderGoVue(t model.GenerateTable, cols []model.GenerateColumn) (string, string) {
	structName := toExported(t.ClassDir)
	if structName == "" {
		structName = toExported(strings.TrimPrefix(t.Name, config.Prefix()))
	}
	var b strings.Builder
	b.WriteString("package generated\n\n")
	b.WriteString(fmt.Sprintf("// %s %s\n", structName, t.TableComment))
	b.WriteString(fmt.Sprintf("type %s struct {\n", structName))
	for _, col := range cols {
		b.WriteString(fmt.Sprintf("\t%s %s `gorm:\"column:%s\" json:\"%s\"`\n",
			toExported(col.ColumnName), goType(col.ColumnType), col.ColumnName, col.ColumnName))
	}
	b.WriteString("}\n\n")
	b.WriteString(fmt.Sprintf("func (%s) TableName() string { return %q }\n", structName, t.Name))
	vue := fmt.Sprintf("<template>\n  <div class=\"%s-lists\">代码生成：%s</div>\n</template>\n", t.ClassDir, t.TableComment)
	return b.String(), vue
}

func toExported(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

func goType(colType string) string {
	t := strings.ToLower(colType)
	switch {
	case strings.Contains(t, "int"):
		return "int64"
	case strings.Contains(t, "decimal"), strings.Contains(t, "float"), strings.Contains(t, "double"):
		return "float64"
	default:
		return "string"
	}
}
