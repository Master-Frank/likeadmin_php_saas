// Package gencrud serves generate_type=1 modules at runtime from
// la_generate_table / la_generate_column so newly generated CRUD works
// without writing PHP or recompiling Go handlers.
package gencrud

import (
	"net/http"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/generator"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/lists"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	actionLists  = "lists"
	actionAdd    = "add"
	actionEdit   = "edit"
	actionDelete = "delete"
	actionDetail = "detail"
)

type spec struct {
	table      model.GenerateTable
	cols       []model.GenerateColumn
	pk         string
	softDelete bool
	deleteCol  string
	tree       bool
	treeID     string
	treePID    string
	allowed    map[string]bool
	rels       []relSpec
}

type relSpec struct {
	Name       string
	Table      string
	Type       string
	LocalKey   string
	ForeignKey string
	Label      string
}

func Match(app, ctrl, action string) bool {
	switch strings.ToLower(action) {
	case actionLists, actionAdd, actionEdit, actionDelete, actionDetail:
	default:
		return false
	}
	return resolve(app, ctrl) != nil
}

func Handle(c *gin.Context) {
	meta := ctxutil.Get(c)
	sp := resolve(meta.App, meta.Controller)
	if sp == nil {
		response.FailCode(c, "controller not exists:"+meta.Controller, response.CodeNotFound, 0)
		return
	}
	switch strings.ToLower(meta.Action) {
	case actionLists:
		doLists(c, sp)
	case actionAdd:
		doAdd(c, sp)
	case actionEdit:
		doEdit(c, sp)
	case actionDelete:
		doDelete(c, sp)
	case actionDetail:
		doDetail(c, sp)
	default:
		response.FailCode(c, "controller not exists:"+meta.Controller, response.CodeNotFound, 0)
	}
}

func resolve(app, ctrl string) *spec {
	if bootstrap.DB == nil || !validIdent(strings.ReplaceAll(ctrl, ".", "_")) {
		return nil
	}
	var tables []model.GenerateTable
	if bootstrap.DB.Find(&tables).Error != nil {
		return nil
	}
	ctrl = strings.ToLower(strings.TrimSpace(ctrl))
	for _, t := range tables {
		if ModuleApp(t.ModuleName) != app {
			continue
		}
		if RouteKey(t) != ctrl {
			continue
		}
		if !validIdent(t.Name) {
			return nil
		}
		var cols []model.GenerateColumn
		bootstrap.DB.Where("table_id = ?", t.ID).Order("id asc").Find(&cols)
		return newSpec(t, cols)
	}
	return nil
}

func newSpec(t model.GenerateTable, cols []model.GenerateColumn) *spec {
	del := util.DecodeJSONMap(t.Delete)
	tree := util.DecodeJSONMap(t.Tree)
	deleteCol := util.ToString(del["name"])
	if deleteCol == "" {
		deleteCol = "delete_time"
	}
	sp := &spec{
		table: t, cols: cols,
		pk: "id", softDelete: util.ToInt(del["type"]) == generator.DeleteSoft,
		deleteCol: deleteCol,
		tree:      t.TemplateType == generator.TemplateTypeTree,
		treeID:    firstNonEmpty(util.ToString(tree["tree_id"]), "id"),
		treePID:   firstNonEmpty(util.ToString(tree["tree_pid"]), "pid"),
		allowed:   map[string]bool{},
		rels:      parseRelations(t),
	}
	for _, col := range cols {
		if validIdent(col.ColumnName) {
			sp.allowed[col.ColumnName] = true
			if col.IsPk == 1 {
				sp.pk = col.ColumnName
			}
		}
	}
	if !validIdent(sp.pk) {
		sp.pk = "id"
	}
	if !validIdent(sp.deleteCol) {
		sp.softDelete = false
	}
	return sp
}

// RouteKey is the request controller, matching PHP permsName().
func RouteKey(t model.GenerateTable) string {
	name := generator.Lower(generator.NoPrefix(t.Name))
	dir := strings.Trim(t.ClassDir, "\\/")
	if dir == "" {
		return name
	}
	return generator.Lower(dir + "." + name)
}

// ModuleApp maps generate_table.module_name onto a Go app prefix.
func ModuleApp(module string) string {
	switch strings.ToLower(strings.TrimSpace(module)) {
	case "", "platform", "platformapi", "admin":
		return "platformapi"
	case "tenant", "tenantapi":
		return "tenantapi"
	case "api", "index", "user":
		return "api"
	default:
		return strings.ToLower(strings.TrimSpace(module))
	}
}

func doLists(c *gin.Context, sp *spec) {
	q, ok := lists.ParseGET(c)
	if !ok {
		return
	}
	db := scoped(c, sp)
	db = applySearch(db, sp, q)
	var count int64
	db.Count(&count)
	if sp.tree {
		q.PageNo = 1
		if q.PageSize < int(count) {
			q.PageSize = int(count)
			if q.PageSize < 1 {
				q.PageSize = 1
			}
		}
		q.Offset = 0
	}
	order := lists.OrderSQL(q, sp.pk+" desc", sp.allowed)
	var rows []map[string]any
	query := db.Select(selectCols(sp)).Order(order).Offset(q.Offset).Limit(q.PageSize)
	if err := query.Find(&rows).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, formatRow(c, sp, row))
	}
	attachRelations(c, sp, out)
	if sp.tree {
		out = util.LinearToTree(out, "children", sp.treeID, sp.treePID, 0)
	}
	response.Lists(c, out, count, q.PageNo, q.PageSize, nil)
}

func requirePOST(c *gin.Context) bool {
	if c != nil && c.Request != nil && c.Request.Method != http.MethodPost {
		response.Fail(c, "请求方式错误，请使用post请求方式")
		return false
	}
	return true
}

func doAdd(c *gin.Context, sp *spec) {
	if !requirePOST(c) {
		return
	}
	p := httpx.Params(c)
	if msg := requiredMsg(sp, p, false); msg != "" {
		response.Fail(c, msg)
		return
	}
	data := writeData(c, sp, p, false)
	if err := session(c).Table(sp.table.Name).Create(data).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "添加成功")
}

func doEdit(c *gin.Context, sp *spec) {
	if !requirePOST(c) {
		return
	}
	p := httpx.Params(c)
	id := httpx.Uint(c, sp.pk)
	if id == 0 {
		response.Fail(c, sp.pk+"不能为空")
		return
	}
	if msg := requiredMsg(sp, p, true); msg != "" {
		response.Fail(c, msg)
		return
	}
	data := writeData(c, sp, p, true)
	if sp.tree && validIdent(sp.treePID) && validIdent(sp.pk) {
		cur := map[string]any{}
		if scoped(c, sp).Select(sp.treePID).Where(sp.pk+" = ?", id).Take(&cur).Error == nil && util.ToInt(cur[sp.treePID]) == 0 {
			data[sp.treePID] = 0
		}
	}
	if msg := treeCycleMsg(c, sp, id, data); msg != "" {
		response.Fail(c, msg)
		return
	}
	q := scoped(c, sp).Where(sp.pk+" = ?", id)
	if err := q.Updates(data).Error; err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "编辑成功")
}

func doDelete(c *gin.Context, sp *spec) {
	if !requirePOST(c) {
		return
	}
	ids := httpx.Uints(c, sp.pk)
	if len(ids) == 0 {
		ids = httpx.Uints(c, "ids")
	}
	if len(ids) == 0 {
		response.Fail(c, sp.pk+"不能为空")
		return
	}
	q := scoped(c, sp).Where(sp.pk+" IN ?", ids)
	var err error
	if sp.softDelete {
		now := util.NowUnix()
		err = q.Updates(map[string]any{sp.deleteCol: now}).Error
	} else {
		err = q.Delete(map[string]any{}).Error
	}
	if err != nil {
		response.Fail(c, err.Error())
		return
	}
	response.SuccessNotice(c, "删除成功")
}

func doDetail(c *gin.Context, sp *spec) {
	id := httpx.QueryUint(c, sp.pk)
	if id == 0 {
		response.Fail(c, sp.pk+"不能为空")
		return
	}
	row := map[string]any{}
	if scoped(c, sp).Where(sp.pk+" = ?", id).Take(&row).Error != nil || len(row) == 0 {
		response.Data(c, []any{})
		return
	}
	formatted := formatRow(c, sp, row)
	attachRelations(c, sp, []map[string]any{formatted})
	response.Data(c, formatted)
}

func session(c *gin.Context) *gorm.DB {
	if db := tenantdb.Use(c); db != nil {
		return db
	}
	return bootstrap.DB
}

func scoped(c *gin.Context, sp *spec) *gorm.DB {
	db := session(c).Table(sp.table.Name)
	if sp.softDelete && sp.allowed[sp.deleteCol] {
		db = db.Where(sp.deleteCol + " IS NULL")
	}
	if sp.allowed["tenant_id"] {
		tid := uint(0)
		if c != nil {
			tid = ctxutil.Get(c).TenantID
		}
		if tid == 0 {
			return db.Where("1 = 0")
		}
		db = db.Where("tenant_id = ?", tid)
	}
	return db
}

func applySearch(db *gorm.DB, sp *spec, q lists.Query) *gorm.DB {
	for _, col := range sp.cols {
		if col.IsQuery == 0 || col.IsPk == 1 || !sp.allowed[col.ColumnName] {
			continue
		}
		name := col.ColumnName
		param := paramName(name)
		qt := strings.ToLower(strings.TrimSpace(col.QueryType))
		if qt == "" {
			qt = "="
		}
		switch qt {
		case "like", "%like%":
			if v := lists.Param(q, param); v != "" {
				db = db.Where(name+" LIKE ?", "%"+v+"%")
			}
		case "%like":
			if v := lists.Param(q, param); v != "" {
				db = db.Where(name+" LIKE ?", "%"+v)
			}
		case "like%":
			if v := lists.Param(q, param); v != "" {
				db = db.Where(name+" LIKE ?", v+"%")
			}
		case "=", "<>", ">", ">=", "<", "<=":
			if _, ok := q.Params[param]; !ok {
				continue
			}
			v := lists.Param(q, param)
			if v == "" {
				continue
			}
			db = db.Where(name+" "+qt+" ?", v)
		case "in":
			if raw, ok := q.Params[param]; ok && raw != nil && util.ToString(raw) != "" {
				db = db.Where(name+" IN ?", toSlice(raw))
			}
		case "between":
			if col.ViewType == "datetime" {
				if q.StartTime != "" && q.EndTime != "" {
					db = db.Where(name+" BETWEEN ? AND ?", q.StartTime, q.EndTime)
				}
			} else {
				start := lists.Param(q, "start")
				end := lists.Param(q, "end")
				if start != "" && end != "" {
					db = db.Where(name+" BETWEEN ? AND ?", start, end)
				}
			}
		case "between_time":
			if q.StartTime != "" && q.EndTime != "" {
				db = db.Where(name+" BETWEEN ? AND ?", q.StartTime, q.EndTime)
			}
		case "find_in_set":
			if v := lists.Param(q, param); v != "" {
				db = db.Where("FIND_IN_SET(?, "+name+")", v)
			}
		}
	}
	return db
}

func selectCols(sp *spec) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(name string) {
		if !sp.allowed[name] || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	add(sp.pk)
	for _, col := range sp.cols {
		if col.IsLists == 1 {
			add(col.ColumnName)
		}
	}
	if sp.tree {
		add(sp.treeID)
		add(sp.treePID)
	}
	for _, name := range []string{"create_time", "update_time"} {
		add(name)
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func writeData(c *gin.Context, sp *spec, p map[string]any, update bool) map[string]any {
	now := util.NowUnix()
	data := map[string]any{}
	for _, col := range sp.cols {
		if !sp.allowed[col.ColumnName] || col.IsPk == 1 {
			continue
		}
		if update && col.IsUpdate != 1 {
			continue
		}
		if !update && col.IsInsert != 1 {
			continue
		}
		if _, ok := p[col.ColumnName]; !ok {
			continue
		}
		val := p[col.ColumnName]
		if col.ColumnType == "int" && col.ViewType == "datetime" {
			val = util.ToInt(val)
		}
		if col.ColumnName == "image" {
			val = filesvc.SetFileURL(c, util.ToString(val))
		}
		data[col.ColumnName] = val
	}
	if !update && sp.allowed["create_time"] {
		data["create_time"] = now
	}
	if sp.allowed["update_time"] {
		data["update_time"] = now
	}
	if !update && sp.allowed["tenant_id"] {
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			data["tenant_id"] = tid
		}
	}
	return data
}

func requiredMsg(sp *spec, p map[string]any, update bool) string {
	for _, col := range sp.cols {
		if col.IsRequired != 1 || col.IsPk == 1 {
			continue
		}
		if update && col.IsUpdate != 1 {
			continue
		}
		if !update && col.IsInsert != 1 {
			continue
		}
		if _, ok := p[col.ColumnName]; !ok || strings.TrimSpace(util.ToString(p[col.ColumnName])) == "" {
			name := col.ColumnComment
			if name == "" {
				name = col.ColumnName
			}
			return name + "不能为空"
		}
	}
	return ""
}

func formatRow(c *gin.Context, sp *spec, row map[string]any) map[string]any {
	out := make(map[string]any, len(row)+4)
	for k, v := range row {
		if b, ok := v.([]byte); ok {
			v = string(b)
		}
		if isTimeCol(sp, k) {
			out[k] = util.FormatDateTime(util.ToInt64(v))
			continue
		}
		if k == "image" {
			out[k] = filesvc.GetImageAttr(c, util.ToString(v))
			continue
		}
		out[k] = v
		if label := dictLabel(sp, k, v); label != "" {
			out[k+"_text"] = label
		}
	}
	return out
}

func isTimeCol(sp *spec, name string) bool {
	if name == "create_time" || name == "update_time" || name == "delete_time" {
		return true
	}
	for _, col := range sp.cols {
		if col.ColumnName == name && col.ViewType == "datetime" {
			return true
		}
	}
	return false
}

func paramName(field string) string {
	if i := strings.LastIndex(field, "."); i >= 0 && i+1 < len(field) {
		return field[i+1:]
	}
	return field
}

func validIdent(name string) bool {
	return lists.Ident(name) == name && name != ""
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func toSlice(v any) []any {
	switch t := v.(type) {
	case []any:
		return t
	case []string:
		out := make([]any, len(t))
		for i, s := range t {
			out[i] = s
		}
		return out
	default:
		s := strings.TrimSpace(util.ToString(v))
		if s == "" {
			return nil
		}
		if strings.Contains(s, ",") {
			parts := strings.Split(s, ",")
			out := make([]any, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			return out
		}
		return []any{s}
	}
}
