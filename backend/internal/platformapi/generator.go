package platformapi

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
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
			ModuleName: "platform", ClassDir: strings.TrimPrefix(name, config.Prefix()),
			GenerateType: 1, Menu: util.EncodeJSON(map[string]any{"pid": 0, "type": 1, "name": comment}),
			Delete: util.EncodeJSON(map[string]any{"type": 1, "name": "delete_time"}),
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
	if err := bootstrap.DB.Model(&model.GenerateTable{}).Where("id = ?", id).Updates(data).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	cols, _ := httpx.Any(c, "table_column").([]any)
	for _, item := range cols {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		colID := uint(util.ToInt(m["id"]))
		if colID == 0 {
			continue
		}
		bootstrap.DB.Model(&model.GenerateColumn{}).Where("id = ?", colID).Updates(map[string]any{
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
		})
	}
	response.SuccessNotice(c, "操作成功")
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
	response.Success(c, "", generateBundle(t, cols))
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
	root := generatorRuntime()
	_ = os.MkdirAll(root, 0755)
	fileName := fmt.Sprintf("curd-%s.zip", time.Now().Format("20060102150405"))
	zipPath := filepath.Join(root, fileName)
	zf, err := os.Create(zipPath)
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	zw := zip.NewWriter(zf)
	for _, id := range ids {
		var t model.GenerateTable
		if bootstrap.DB.First(&t, id).Error != nil {
			continue
		}
		var cols []model.GenerateColumn
		bootstrap.DB.Where("table_id = ?", t.ID).Find(&cols)
		for _, f := range generateBundle(t, cols) {
			name := t.ClassDir + "/" + util.ToString(f["name"])
			w, err := zw.Create(name)
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte(util.ToString(f["content"])))
		}
	}
	_ = zw.Close()
	_ = zf.Close()
	cache.Set("curd_file_name"+fileName, fileName, time.Hour)
	fileURL := ctxutil.Domain(c) + "/platformapi/tools.generator/download?file=" + fileName
	response.Result(c, 1, 1, "操作成功", gin.H{"file": fileURL})
}

func GeneratorDownload(c *gin.Context) {
	fileName := httpx.Str(c, "file")
	if fileName == "" {
		response.Fail(c, "下载失败")
		return
	}
	if _, ok := cache.Get("curd_file_name" + fileName); !ok {
		response.Fail(c, "请重新生成代码")
		return
	}
	zipPath := filepath.Join(generatorRuntime(), fileName)
	if _, err := os.Stat(zipPath); err != nil {
		response.Fail(c, "下载失败")
		return
	}
	cache.Del("curd_file_name" + fileName)
	c.FileAttachment(zipPath, "likeadmin-curd.zip")
}

func generatorRoot() string {
	return generatorRuntime()
}

func generatorRuntime() string {
	pub := config.C.App.PublicDir
	if pub == "" {
		pub = "server/public"
	}
	return filepath.Join(filepath.Dir(pub), "runtime", "generate")
}

func GeneratorGetModels(c *gin.Context) {
	root := filepath.Join(filepath.Dir(config.C.App.PublicDir), "app", "common", "model")
	out := []string{}
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() || !strings.HasSuffix(path, ".php") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = strings.TrimSuffix(rel, ".php")
		parts := strings.Split(rel, string(os.PathSeparator))
		name := `\app\common\model`
		for _, p := range parts {
			name += `\` + p
		}
		out = append(out, name)
		return nil
	})
	response.Data(c, out)
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
			ColumnType: col.ColumnType, IsPk: pk, IsRequired: req,
			IsInsert: ins, IsUpdate: upd, IsLists: lists, IsQuery: query,
			QueryType: "=", ViewType: "input", CreateTime: now,
		})
	}
}

func generateBundle(t model.GenerateTable, cols []model.GenerateColumn) []map[string]any {
	name := toExported(t.ClassDir)
	if name == "" {
		name = toExported(strings.TrimPrefix(t.Name, config.Prefix()))
	}
	mod := t.ModuleName
	if mod == "" {
		mod = "admin"
	}
	author := t.Author
	if author == "" {
		author = "likeadmin"
	}
	comment := t.TableComment
	if comment == "" {
		comment = name
	}
	var fields strings.Builder
	var vueCols strings.Builder
	pk := "id"
	for _, col := range cols {
		fields.WriteString(fmt.Sprintf("    public $%s;\n", col.ColumnName))
		if col.IsPk == 1 {
			pk = col.ColumnName
		}
		if col.IsLists == 1 {
			vueCols.WriteString(fmt.Sprintf("      { label: '%s', field: '%s' },\n", firstNonEmpty(col.ColumnComment, col.ColumnName), col.ColumnName))
		}
	}
	snake := strings.TrimPrefix(t.Name, config.Prefix())
	if snake == "" {
		snake = t.ClassDir
	}
	phpNS := "app\\" + mod
	ctrl := fmt.Sprintf("<?php\nnamespace %s\\controller%s;\n\nuse %s\\controller\\BaseAdminController;\nuse %s\\lists%s\\%sLists;\nuse %s\\logic%s\\%sLogic;\nuse %s\\validate%s\\%sValidate;\n\n/** %s */\nclass %sController extends BaseAdminController\n{\n    public function lists()\n    {\n        return $this->dataLists(new %sLists());\n    }\n    public function add()\n    {\n        $params = (new %sValidate())->post()->goCheck('add');\n        %sLogic::add($params);\n        return $this->success('添加成功', [], 1, 1);\n    }\n    public function edit()\n    {\n        $params = (new %sValidate())->post()->goCheck('edit');\n        %sLogic::edit($params);\n        return $this->success('编辑成功', [], 1, 1);\n    }\n    public function delete()\n    {\n        $params = (new %sValidate())->post()->goCheck('delete');\n        %sLogic::delete($params);\n        return $this->success('删除成功', [], 1, 1);\n    }\n    public function detail()\n    {\n        $params = (new %sValidate())->goCheck('detail');\n        return $this->data(%sLogic::detail($params));\n    }\n}\n",
		phpNS, classDirNS(t.ClassDir), phpNS, phpNS, classDirNS(t.ClassDir), name, phpNS, classDirNS(t.ClassDir), name, phpNS, classDirNS(t.ClassDir), name, comment, name, name, name, name, name, name, name, name, name, name)
	lists := fmt.Sprintf("<?php\nnamespace %s\\lists%s;\n\nuse %s\\lists\\BaseAdminDataLists;\nuse app\\common\\model%s\\%s;\n\n/** %s列表 */\nclass %sLists extends BaseAdminDataLists\n{\n    public function lists(): array\n    {\n        return %s::limit($this->limitOffset, $this->limitLength)->order('%s desc')->select()->toArray();\n    }\n    public function count(): int\n    {\n        return %s::count();\n    }\n}\n", phpNS, classDirNS(t.ClassDir), phpNS, classDirNS(t.ClassDir), name, comment, name, name, pk, name)
	modelPHP := fmt.Sprintf("<?php\nnamespace app\\common\\model%s;\n\nuse app\\common\\model\\BaseModel;\nuse think\\model\\concern\\SoftDelete;\n\n/** %s */\nclass %s extends BaseModel\n{\n    use SoftDelete;\n    protected $name = '%s';\n    protected $deleteTime = 'delete_time';\n%s}\n", classDirNS(t.ClassDir), comment, name, snake, fields.String())
	validate := fmt.Sprintf("<?php\nnamespace %s\\validate%s;\n\nuse app\\common\\validate\\BaseValidate;\n\nclass %sValidate extends BaseValidate\n{\n    protected $rule = ['id' => 'require'];\n    public function sceneAdd() { return $this->remove('id', true); }\n    public function sceneEdit() { return $this; }\n    public function sceneDelete() { return $this->only(['id']); }\n    public function sceneDetail() { return $this->only(['id']); }\n}\n", phpNS, classDirNS(t.ClassDir), name)
	logic := fmt.Sprintf("<?php\nnamespace %s\\logic%s;\n\nuse app\\common\\logic\\BaseLogic;\nuse app\\common\\model%s\\%s;\n\nclass %sLogic extends BaseLogic\n{\n    public static function add(array $params) { %s::create($params); return true; }\n    public static function edit(array $params) { %s::update($params); return true; }\n    public static function delete(array $params) { %s::destroy($params['id']); return true; }\n    public static function detail(array $params) { return %s::findOrEmpty($params['id'])->toArray(); }\n}\n", phpNS, classDirNS(t.ClassDir), classDirNS(t.ClassDir), name, name, name, name, name, name)
	vueAPI := fmt.Sprintf("import request from '@/utils/request'\n\nexport function api%sLists(params: any) {\n  return request.get({ url: '/%s/%s/lists', params })\n}\nexport function api%sAdd(params: any) {\n  return request.post({ url: '/%s/%s/add', params })\n}\nexport function api%sEdit(params: any) {\n  return request.post({ url: '/%s/%s/edit', params })\n}\nexport function api%sDelete(params: any) {\n  return request.post({ url: '/%s/%s/delete', params })\n}\nexport function api%sDetail(params: any) {\n  return request.get({ url: '/%s/%s/detail', params })\n}\n", name, mod+"api", snake, name, mod+"api", snake, name, mod+"api", snake, name, mod+"api", snake, name, mod+"api", snake)
	vueIndex := fmt.Sprintf("<template>\n  <div class=\"%s-lists\">\n    <el-table :data=\"lists\">\n%s    </el-table>\n  </div>\n</template>\n<script lang=\"ts\" setup>\nimport { api%sLists } from '@/api/%s'\nconst lists = ref([])\n</script>\n", snake, vueCols.String(), name, snake)
	vueEdit := fmt.Sprintf("<template>\n  <el-form :model=\"form\">\n    <el-form-item label=\"%s\"><el-input v-model=\"form.%s\" /></el-form-item>\n  </el-form>\n</template>\n<script lang=\"ts\" setup>\nconst form = reactive({ %s: '' })\n</script>\n", comment, pk, pk)
	sql := fmt.Sprintf("-- menu for %s\n-- author: %s\n", comment, author)
	return []map[string]any{
		{"name": name + "Controller.php", "type": "php", "content": ctrl},
		{"name": name + "Lists.php", "type": "php", "content": lists},
		{"name": name + ".php", "type": "php", "content": modelPHP},
		{"name": name + "Validate.php", "type": "php", "content": validate},
		{"name": name + "Logic.php", "type": "php", "content": logic},
		{"name": snake + ".ts", "type": "typescript", "content": vueAPI},
		{"name": "index.vue", "type": "vue", "content": vueIndex},
		{"name": "edit.vue", "type": "vue", "content": vueEdit},
		{"name": snake + ".sql", "type": "sql", "content": sql},
	}
}

func classDirNS(dir string) string {
	dir = strings.Trim(dir, "\\/")
	if dir == "" {
		return ""
	}
	return "\\" + strings.ReplaceAll(dir, "/", "\\")
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
