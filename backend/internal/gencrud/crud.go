// Package gencrud serves generate_type=1 modules at runtime from
// la_generate_table / la_generate_column so newly generated CRUD works
// without writing PHP or recompiling Go handlers.
package gencrud

import (
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

// RouteKey is the HTTP controller, matching PHP VueApiGenerator::getRouteContent.
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
	return response.RequirePOST(c)
}

func doAdd(c *gin.Context, sp *spec) {
	if !requirePOST(c) {
		return
	}
	if !requireTenantWrite(c, sp) {
		return
	}
	p := httpx.Body(c)
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
	if !requireTenantWrite(c, sp) {
		return
	}
	p := httpx.Body(c)
	if !httpx.BodyPresent(c, sp.pk) {
		response.Fail(c, sp.pk+"不能为空")
		return
	}
	id := httpx.BodyUint(c, sp.pk)
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
	if !requireTenantWrite(c, sp) {
		return
	}
	ids := httpx.BodyUints(c, sp.pk)
	if len(ids) == 0 {
		ids = httpx.BodyUints(c, "ids")
	}
	if len(ids) == 0 {
		response.Fail(c, sp.pk+"不能为空")
		return
	}
	q := scoped(c, sp).Where(sp.pk+" IN ?", ids)
	var err error
	if sp.softDelete {
		now := util.NowUnix()
		fields := map[string]any{sp.deleteCol: now}
		if sp.allowed["update_time"] || tableHasColumn(bootstrap.DB, sp.table.Name, "update_time") {
			fields["update_time"] = now
		}
		err = q.Updates(fields).Error
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
	if !httpx.QueryPresent(c, sp.pk) {
		response.Fail(c, sp.pk+"不能为空")
		return
	}
	id := httpx.QueryUint(c, sp.pk)
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

// requireTenantWrite rejects platform/tenant writes that would land on tenant_id=0.
func requireTenantWrite(c *gin.Context, sp *spec) bool {
	if sp == nil || !sp.allowed["tenant_id"] {
		return true
	}
	meta := ctxutil.Get(c)
	if meta.Source == ctxutil.SourcePlatform && meta.TenantID == 0 {
		response.Fail(c, "请选择租户标识")
		return false
	}
	if meta.TenantID == 0 {
		response.Fail(c, "参数缺失")
		return false
	}
	return true
}

func scoped(c *gin.Context, sp *spec) *gorm.DB {
	db := session(c).Table(sp.table.Name)
	// PHP generated models use SoftDelete whenever delete.type=1,
	// even if delete_time is not a listed generate_column.
	if sp.softDelete {
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
			// PHP ListsSearchTrait %like%: skip when empty() — "0" does not LIKE.
			if lists.PHPTruthy(q, param) {
				db = db.Where(name+" LIKE ?", "%"+lists.Param(q, param)+"%")
			}
		case "%like":
			if lists.PHPTruthy(q, param) {
				db = db.Where(name+" LIKE ?", "%"+lists.Param(q, param))
			}
		case "like%":
			if lists.PHPTruthy(q, param) {
				db = db.Where(name+" LIKE ?", lists.Param(q, param)+"%")
			}
		case "=", "<>", ">", ">=", "<", "<=":
			// PHP '=' family: skip only when !isset or == ''.
			if lists.HasParam(q, param) {
				db = db.Where(name+" "+qt+" ?", lists.Param(q, param))
			}
		case "in":
			if lists.HasParam(q, param) {
				db = db.Where(name+" IN ?", toSlice(q.Params[param]))
			}
		case "between":
			if col.ViewType == "datetime" || col.ViewType == "datetime2" {
				if start, end, ok := parseSearchTimeRange(q.StartTime, q.EndTime); ok {
					db = db.Where(name+" BETWEEN ? AND ?", start, end)
				}
			} else if lists.PHPTruthy(q, "start") && lists.PHPTruthy(q, "end") {
				// PHP empty($this->start) || empty($this->end)
				db = db.Where(name+" BETWEEN ? AND ?", lists.Param(q, "start"), lists.Param(q, "end"))
			}
		case "between_time":
			if start, end, ok := parseSearchTimeRange(q.StartTime, q.EndTime); ok {
				db = db.Where(name+" BETWEEN ? AND ?", start, end)
			}
		case "find_in_set":
			if lists.HasParam(q, param) {
				db = db.Where("FIND_IN_SET(?, "+name+")", lists.Param(q, param))
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
	// PHP ListsGenerator::getFieldDataContent only emits is_lists (+ tree keys).
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

// coerceColumnValue mirrors PHP LogicGenerator: int+datetime columns use strtotime.
// Other int columns follow ThinkPHP/MySQL int cast (blank/bool → 0/1).
func coerceColumnValue(col model.GenerateColumn, val any) any {
	if col.ColumnType == "int" && col.ViewType == "datetime" {
		return util.ParseDateTime(util.ToString(val))
	}
	if col.ColumnType == "int" {
		switch t := val.(type) {
		case bool:
			if t {
				return 1
			}
			return 0
		case string:
			return util.ToInt(strings.TrimSpace(t))
		}
	}
	return val
}

// parseSearchTimeRange mirrors PHP BaseDataLists::initSearch strtotime + between_time.
func parseSearchTimeRange(start, end string) (int64, int64, bool) {
	s, e := util.ParseDateTime(start), util.ParseDateTime(end)
	if s == 0 || e == 0 {
		return 0, 0, false
	}
	return s, e, true
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
		val := coerceColumnValue(col, p[col.ColumnName])
		if isCheckboxCol(col) {
			val = joinCheckbox(val)
		} else if isImageCol(sp, col.ColumnName) {
			val = filesvc.SetFileURL(c, util.ToString(val))
		} else if isEditorCol(col) {
			val = filesvc.ClearContentDomains(c, util.ToString(val))
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
		// PHP ValidateGenerator edit scene requires every is_required column,
		// including those with is_update=0. Add still honors is_insert.
		if !update && col.IsInsert != 1 {
			continue
		}
		if _, ok := p[col.ColumnName]; !ok || isRequireEmpty(p[col.ColumnName]) {
			name := col.ColumnComment
			if name == "" {
				name = col.ColumnName
			}
			// PHP ValidateGenerator message is the column comment only.
			return name
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
			if ts, ok := asUnixTime(v); ok {
				// PHP generated lists/detail return raw unix ints; Vue timeFormat()
				// only treats length-10/13 values as timestamps.
				out[k] = ts
				continue
			}
		}
		if isImageCol(sp, k) {
			out[k] = filesvc.GetImageAttr(c, util.ToString(v))
			continue
		}
		if isEditorColName(sp, k) {
			out[k] = filesvc.RewriteContentDomains(c, util.ToString(v))
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

// asUnixTime keeps numeric datetime columns as unix seconds (PHP toArray()).
func asUnixTime(v any) (int64, bool) {
	if b, ok := v.([]byte); ok {
		v = string(b)
	}
	switch n := v.(type) {
	case nil:
		return 0, false
	case int, int8, int16, int32, int64, uint, uint32, uint64, float32, float64:
		return util.ToInt64(n), true
	case string:
		s := strings.TrimSpace(n)
		if s == "" {
			return 0, false
		}
		for _, r := range s {
			if r < '0' || r > '9' {
				return 0, false
			}
		}
		return util.ToInt64(s), true
	default:
		return 0, false
	}
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
