package platformapi

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/generator"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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
		db = db.Where("table_name LIKE ? OR table_comment LIKE ?", "%"+n+"%", "%"+n+"%")
	}
	if cmt := lists.Param(q, "table_comment"); cmt != "" {
		db = db.Where("table_comment LIKE ?", "%"+cmt+"%")
	}
	var count int64
	db.Count(&count)
	var rows []model.GenerateTable
	db.Order("id desc").Offset(q.Offset).Limit(q.PageSize).Find(&rows)
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"id": r.ID, "table_name": r.Name, "table_comment": r.TableComment,
			"template_type": r.TemplateType, "template_type_desc": generatorTemplateTypeDesc(r.TemplateType),
			"generate_type": r.GenerateType, "module_name": r.ModuleName,
			"author": r.Author, "remark": r.Remark, "create_time": util.FormatDateTime(r.CreateTime),
		})
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func GeneratorSelectTable(c *gin.Context) {
	tables := httpx.Any(c, "table")
	if tables == nil {
		response.Fail(c, "参数缺失")
		return
	}
	arr, ok := tables.([]any)
	if !ok {
		response.Fail(c, "参数类型错误")
		return
	}
	if len(arr) == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	adminID := ctxutil.Get(c).AdminID
	now := util.NowUnix()
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			response.Fail(c, "参数缺失")
			return
		}
		if _, hasName := m["name"]; !hasName {
			if _, has := m["table_name"]; !has {
				response.Fail(c, "参数缺失")
				return
			}
		}
		if _, hasComment := m["comment"]; !hasComment {
			if _, has := m["table_comment"]; !has {
				response.Fail(c, "参数缺失")
				return
			}
		}
		name := util.ToString(m["name"])
		if name == "" {
			name = util.ToString(m["table_name"])
		}
		comment := util.ToString(m["comment"])
		if comment == "" {
			comment = util.ToString(m["table_comment"])
		}
		var n int64
		bootstrap.DB.Raw("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?", name).Scan(&n)
		if n == 0 {
			response.Fail(c, "当前数据库不存在"+name+"表")
			return
		}
		gt := model.GenerateTable{
			Name: name, TableComment: comment, Author: "likeadmin",
			ModuleName: "platform", ClassDir: "",
			TemplateType: 0, GenerateType: 0,
			Menu:      util.EncodeJSON(map[string]any{"pid": 0, "type": 0, "name": comment}),
			Delete:    util.EncodeJSON(map[string]any{"type": 0, "name": "delete_time"}),
			Relations: util.EncodeJSON([]any{}), Tree: util.EncodeJSON(map[string]any{}),
			AdminID: adminID, CreateTime: now,
		}
		if err := bootstrap.DB.Create(&gt).Error; err != nil {
			response.Fail(c, err.Error())
			return
		}
		syncColumns(gt.ID, name)
	}
	response.SuccessNotice(c, "操作成功")
}

func GeneratorDetail(c *gin.Context) {
	if httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	var t model.GenerateTable
	if bootstrap.DB.First(&t, httpx.Uint(c, "id")).Error != nil {
		response.Fail(c, "信息不存在")
		return
	}
	var cols []model.GenerateColumn
	bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
	response.Data(c, formatGeneratorDetail(t, cols))
}

func GeneratorSyncColumn(c *gin.Context) {
	if _, ok := httpx.Params(c)["id"]; !ok || httpx.Uint(c, "id") == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	id := httpx.Uint(c, "id")
	var t model.GenerateTable
	if bootstrap.DB.First(&t, id).Error != nil {
		response.Fail(c, "信息不存在")
		return
	}
	bootstrap.DB.Where("table_id = ?", id).Delete(&model.GenerateColumn{})
	syncColumns(id, t.Name)
	response.SuccessNotice(c, "操作成功")
}

func GeneratorDelete(c *gin.Context) {
	ids := httpx.Uints(c, "id")
	if len(ids) == 0 {
		ids = httpx.Uints(c, "ids")
	}
	if id := httpx.Uint(c, "id"); id > 0 && len(ids) == 0 {
		ids = []uint{id}
	}
	if len(ids) == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	for _, id := range ids {
		var t model.GenerateTable
		if bootstrap.DB.First(&t, id).Error != nil {
			response.Fail(c, "信息不存在")
			return
		}
	}
	bootstrap.DB.Where("id IN ?", ids).Delete(&model.GenerateTable{})
	bootstrap.DB.Where("table_id IN ?", ids).Delete(&model.GenerateColumn{})
	response.SuccessNotice(c, "操作成功")
}

func GeneratorEdit(c *gin.Context) {
	p := httpx.Params(c)
	if msg := util.GeneratorEditCheck(p); msg != "" {
		response.Fail(c, msg)
		return
	}
	id := httpx.Uint(c, "id")
	var t model.GenerateTable
	if bootstrap.DB.First(&t, id).Error != nil {
		response.Fail(c, "信息不存在")
		return
	}
	now := util.NowUnix()
	data := map[string]any{
		"table_name": httpx.Str(c, "table_name"), "table_comment": httpx.Str(c, "table_comment"),
		"template_type": httpx.Int(c, "template_type"), "author": httpx.Str(c, "author"),
		"remark": httpx.Str(c, "remark"), "generate_type": httpx.Int(c, "generate_type"),
		"module_name": httpx.Str(c, "module_name"), "class_dir": httpx.Str(c, "class_dir"),
		"class_comment": httpx.Str(c, "class_comment"), "update_time": now,
	}
	if v := httpx.Any(c, "menu"); v != nil {
		data["menu"] = util.EncodeJSON(v)
	}
	if v := httpx.Any(c, "delete"); v != nil {
		data["delete"] = util.EncodeJSON(v)
	}
	if v := httpx.Any(c, "tree"); v != nil {
		data["tree"] = util.EncodeJSON(v)
	}
	if v := httpx.Any(c, "relations"); v != nil {
		data["relations"] = util.EncodeJSON(v)
	}
	cols, _ := httpx.Any(c, "table_column").([]any)
	err := bootstrap.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.GenerateTable{}).Where("id = ?", id).Updates(data).Error; err != nil {
			return err
		}
		for _, item := range cols {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			colID := uint(util.ToInt(m["id"]))
			if colID == 0 {
				continue
			}
			var n int64
			tx.Model(&model.GenerateColumn{}).Where("id = ? AND table_id = ?", colID, id).Count(&n)
			if n == 0 {
				return fmt.Errorf("字段不存在")
			}
			if err := tx.Model(&model.GenerateColumn{}).Where("id = ? AND table_id = ?", colID, id).Updates(map[string]any{
				"column_comment": util.ToString(m["column_comment"]),
				"is_required":    util.ToInt(m["is_required"]),
				"is_insert":      util.ToInt(m["is_insert"]),
				"is_update":      util.ToInt(m["is_update"]),
				"is_lists":       util.ToInt(m["is_lists"]),
				"is_query":       util.ToInt(m["is_query"]),
				"query_type":     util.ToString(m["query_type"]),
				"view_type":      util.ToString(m["view_type"]),
				"dict_type":      util.ToString(m["dict_type"]),
				"update_time":    now,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "操作成功")
}

func GeneratorPreview(c *gin.Context) {
	id := httpx.Uint(c, "id")
	var t model.GenerateTable
	if bootstrap.DB.First(&t, id).Error != nil {
		response.Fail(c, "信息不存在")
		return
	}
	var cols []model.GenerateColumn
	bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
	response.Data(c, generator.Preview(t, cols))
}

func GeneratorGenerate(c *gin.Context) {
	ids := httpx.Uints(c, "id")
	if len(ids) == 0 {
		ids = httpx.Uints(c, "ids")
	}
	if id := httpx.Uint(c, "id"); id > 0 && len(ids) == 0 {
		ids = []uint{id}
	}
	if len(ids) == 0 {
		response.Fail(c, "参数缺失")
		return
	}
	for _, id := range ids {
		var t model.GenerateTable
		if bootstrap.DB.First(&t, id).Error != nil {
			response.Fail(c, "信息不存在")
			return
		}
	}
	_ = generator.ClearRuntime()
	needZip := false
	var zipFiles []generator.File
	for _, id := range ids {
		var t model.GenerateTable
		if bootstrap.DB.First(&t, id).Error != nil {
			continue
		}
		var cols []model.GenerateColumn
		bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
		files := generator.Build(t, cols)
		if generator.IsZip(t) {
			needZip = true
			if err := generator.WriteRuntime(files); err != nil {
				response.Fail(c, err.Error())
				return
			}
			zipFiles = append(zipFiles, files...)
		} else if err := generator.WriteModule(t, files); err != nil {
			response.Fail(c, err.Error())
			return
		}
		if generator.IsAutoMenu(t) {
			for _, f := range files {
				if f.Name == "menu.sql" {
					_ = generator.ApplyMenuSQL(bootstrap.DB, f.Content)
				}
			}
		}
	}
	fileURL := ""
	if needZip {
		root := generator.RuntimeDir()
		_ = os.MkdirAll(root, 0755)
		fileName := fmt.Sprintf("curd-%s.zip", time.Now().Format("20060102150405"))
		zipPath := filepath.Join(root, fileName)
		if err := generator.ZipRuntime(zipPath); err != nil {
			response.Fail(c, err.Error())
			return
		}
		cache.Set("curd_file_name"+fileName, fileName, time.Hour)
		fileURL = ctxutil.Domain(c) + "/platformapi/tools.generator/download?file=" + fileName
	}
	response.Result(c, 1, 1, "操作成功", gin.H{"file": fileURL})
}

func GeneratorDownload(c *gin.Context) {
	fileName := httpx.Str(c, "file")
	if fileName == "" {
		response.Fail(c, "下载失败")
		return
	}
	zipPath := filepath.Join(generator.RuntimeDir(), fileName)
	if _, err := os.Stat(zipPath); err != nil {
		response.Fail(c, "下载失败")
		return
	}
	if _, ok := cache.Get("curd_file_name" + fileName); !ok {
		// File still on disk after a prior download (strangler pair hits the same URL twice).
		c.FileAttachment(zipPath, "likeadmin-curd.zip")
		return
	}
	cache.Del("curd_file_name" + fileName)
	c.FileAttachment(zipPath, "likeadmin-curd.zip")
}

func GeneratorGetModels(c *gin.Context) {
	out := scanPHPModels(filepath.Join(filepath.Dir(config.C.App.PublicDir), "app", "common", "model"))
	if len(out) == 0 {
		// Go-only deploy: keep the relation picker usable without the PHP tree.
		out = scanGoModels(filepath.Join(filepath.Dir(filepath.Dir(config.C.App.PublicDir)), "backend", "internal", "model"))
	}
	response.Result(c, 1, 1, "", out)
}

func scanPHPModels(root string) []string {
	out := []string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if entry.IsDir() {
			sub, err := os.ReadDir(filepath.Join(root, entry.Name()))
			if err != nil {
				continue
			}
			for _, item := range sub {
				if item.IsDir() || !strings.HasSuffix(item.Name(), ".php") {
					continue
				}
				out = append(out, `\app\common\model\`+entry.Name()+`\`+strings.TrimSuffix(item.Name(), ".php"))
			}
			continue
		}
		base := strings.TrimSuffix(entry.Name(), ".php")
		if base == "BaseModel" || !strings.HasSuffix(entry.Name(), ".php") {
			continue
		}
		out = append(out, `\app\common\model\`+base)
	}
	return out
}

var goStructRe = regexp.MustCompile(`(?m)^type\s+([A-Z][A-Za-z0-9]+)\s+struct\b`)

// phpModelDir maps Go model type names to PHP app/common/model subdirs.
var phpModelDir = map[string]string{
	"Article": "article", "ArticleCate": "article", "ArticleCollect": "article",
	"OfficialAccountReply": "channel",
	"User":                 "user", "UserAuth": "user", "UserAccountLog": "user", "UserSession": "user",
	"PayConfig": "pay", "TenantPayConfig": "pay", "PayWay": "pay", "TenantPayWay": "pay",
	"Tenant": "tenant",
	"Jobs":   "dept", "Dept": "dept", "TenantJobs": "dept", "TenantDept": "dept",
	"DecoratePage": "decorate", "DecorateTabbar": "decorate",
	"DictData": "dict", "DictType": "dict",
	"File": "file", "FileCate": "file", "TenantFile": "file", "TenantFileCate": "file",
	"RechargeOrder":  "recharge",
	"GenerateColumn": "tools", "GenerateTable": "tools",
	"RefundRecord": "refund", "RefundLog": "refund",
	"NoticeSetting": "notice", "TenantNoticeRecord": "notice", "SmsLog": "notice",
	"NoticeRecord": "notice", "TenantNoticeSetting": "notice", "TenantSmsLog": "notice",
	"Admin": "auth", "AdminJobs": "auth", "AdminDept": "auth", "AdminRole": "auth",
	"AdminSession": "auth", "SystemMenu": "auth", "SystemRole": "auth", "SystemRoleMenu": "auth",
	"TenantAdmin": "auth", "TenantAdminRole": "auth", "TenantAdminJobs": "auth",
	"TenantAdminDept": "auth", "TenantAdminSession": "auth", "TenantSystemMenu": "auth",
	"TenantSystemRole": "auth", "TenantSystemRoleMenu": "auth",
}

func scanGoModels(root string) []string {
	out := []string{}
	entries, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			continue
		}
		for _, m := range goStructRe.FindAllSubmatch(raw, -1) {
			typ := string(m[1])
			path := `\app\common\model\` + typ
			if dir := phpModelDir[typ]; dir != "" {
				path = `\app\common\model\` + dir + `\` + typ
			}
			if seen[path] {
				continue
			}
			seen[path] = true
			out = append(out, path)
		}
	}
	return out
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
	skip := map[string]bool{"id": true, "create_time": true, "update_time": true, "delete_time": true}
	for _, col := range cols {
		pk, req := 0, 0
		if col.ColumnKey == "PRI" {
			pk = 1
		}
		if col.IsNullable == "NO" && pk == 0 && !skip[col.ColumnName] {
			req = 1
		}
		ins, upd, lists, query := 0, 0, 0, 0
		if !skip[col.ColumnName] {
			ins, upd, lists, query = 1, 1, 1, 1
		}
		bootstrap.DB.Create(&model.GenerateColumn{
			TableID: tableID, ColumnName: col.ColumnName, ColumnComment: col.ColumnComment,
			ColumnType: util.DbFieldType(col.ColumnType), IsPk: pk, IsRequired: req,
			IsInsert: ins, IsUpdate: upd, IsLists: lists, IsQuery: query,
			QueryType: "=", ViewType: "input", CreateTime: now,
		})
	}
}

func generatorTemplateTypeDesc(t int) string {
	if t == 1 {
		return "树表(增删改查)"
	}
	return "单表(增删改查)"
}

func formatGeneratorDetail(t model.GenerateTable, cols []model.GenerateColumn) map[string]any {
	menu := util.DecodeJSONMap(t.Menu)
	del := util.DecodeJSONMap(t.Delete)
	tree := util.DecodeJSONMap(t.Tree)
	relRaw := util.DecodeJSON(t.Relations)
	rels := make([]any, 0)
	if arr, ok := relRaw.([]any); ok {
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			typ := util.ToString(m["type"])
			if typ == "" {
				typ = "has_one"
			}
			local := util.ToString(m["local_key"])
			if local == "" {
				local = "id"
			}
			foreign := util.ToString(m["foreign_key"])
			if foreign == "" {
				foreign = "id"
			}
			rels = append(rels, map[string]any{
				"name": util.ToString(m["name"]), "model": util.ToString(m["model"]),
				"type": typ, "local_key": local, "foreign_key": foreign,
			})
		}
	}
	menuName := util.ToString(menu["name"])
	if menuName == "" {
		menuName = t.TableComment
	}
	delName := util.ToString(del["name"])
	if delName == "" {
		delName = "delete_time"
	}
	columns := make([]map[string]any, 0, len(cols))
	for _, col := range cols {
		columns = append(columns, map[string]any{
			"id": col.ID, "table_id": col.TableID, "column_name": col.ColumnName,
			"column_comment": col.ColumnComment, "column_type": col.ColumnType,
			"is_required": col.IsRequired, "is_pk": col.IsPk, "is_insert": col.IsInsert,
			"is_update": col.IsUpdate, "is_lists": col.IsLists, "is_query": col.IsQuery,
			"query_type": col.QueryType, "view_type": col.ViewType, "dict_type": col.DictType,
			"create_time": util.FormatDateTime(col.CreateTime),
			"update_time": util.FormatDateTimeOrNil(col.UpdateTime),
		})
	}
	return map[string]any{
		"id": t.ID, "table_name": t.Name, "table_comment": t.TableComment,
		"template_type": t.TemplateType, "author": t.Author, "remark": t.Remark,
		"generate_type": t.GenerateType, "module_name": t.ModuleName, "class_dir": t.ClassDir,
		"class_comment": t.ClassComment, "admin_id": t.AdminID,
		"menu": map[string]any{
			"pid": util.ToInt(menu["pid"]), "type": util.ToInt(menu["type"]), "name": menuName,
		},
		"delete": map[string]any{
			"type": util.ToInt(del["type"]), "name": delName,
		},
		"tree": map[string]any{
			"tree_id":   util.ToString(tree["tree_id"]),
			"tree_pid":  util.ToString(tree["tree_pid"]),
			"tree_name": util.ToString(tree["tree_name"]),
		},
		"relations":    rels,
		"table_column": columns,
		"create_time":  util.FormatDateTime(t.CreateTime),
		"update_time":  util.FormatDateTimeOrNil(t.UpdateTime),
	}
}
