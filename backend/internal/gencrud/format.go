package gencrud

import (
	"strings"
	"unicode"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/generator"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func parseRelations(t model.GenerateTable) []relSpec {
	raw := util.DecodeJSON(t.Relations)
	arr, _ := raw.([]any)
	out := make([]relSpec, 0, len(arr))
	for _, item := range arr {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		name := listsIdent(util.ToString(m["name"]))
		table := modelToTable(util.ToString(m["model"]))
		local := listsIdent(firstNonEmpty(util.ToString(m["local_key"]), "id"))
		foreign := listsIdent(firstNonEmpty(util.ToString(m["foreign_key"]), "id"))
		relType := util.ToString(m["type"])
		if name == "" || table == "" || local == "" || foreign == "" {
			continue
		}
		// PHP ModelGenerator skips when php/model/{type} stub is missing.
		if relType != "has_one" && relType != "has_many" {
			continue
		}
		out = append(out, relSpec{
			Name: name, Table: table,
			Type:       relType,
			LocalKey:   local,
			ForeignKey: foreign,
			Label:      listsIdent(firstNonEmpty(util.ToString(m["label"]), util.ToString(m["field"]))),
		})
	}
	return out
}

func modelToTable(modelName string) string {
	modelName = strings.ReplaceAll(strings.TrimSpace(modelName), "\\", "/")
	if modelName == "" {
		return ""
	}
	base := modelName
	if i := strings.LastIndex(modelName, "/"); i >= 0 {
		base = modelName[i+1:]
	}
	snake := toSnake(base)
	if strings.HasPrefix(snake, config.Prefix()) {
		if validIdent(snake) {
			return snake
		}
		return ""
	}
	name := config.Prefix() + snake
	if validIdent(name) {
		return name
	}
	return ""
}

func toSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		if r == '-' || r == ' ' {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	return generator.Lower(b.String())
}

func listsIdent(s string) string {
	s = strings.TrimSpace(s)
	if !validIdent(s) {
		return ""
	}
	return s
}

func isImageCol(sp *spec, name string) bool {
	if name == "image" {
		// PHP BaseModel::getImageAttr / setImageAttr is bound to column "image".
		return true
	}
	for _, col := range sp.cols {
		if col.ColumnName != name {
			continue
		}
		switch strings.ToLower(col.ViewType) {
		case "image", "imageselect", "file":
			return true
		}
	}
	return false
}

func isCheckboxCol(col model.GenerateColumn) bool {
	return strings.EqualFold(col.ViewType, "checkbox")
}

func isEditorCol(col model.GenerateColumn) bool {
	return strings.EqualFold(col.ViewType, "editor")
}

func isEditorColName(sp *spec, name string) bool {
	for _, col := range sp.cols {
		if col.ColumnName == name && isEditorCol(col) {
			return true
		}
	}
	return false
}

// joinCheckbox matches VueEditGenerator getCheckBoxJoinContent: array.join(",").
func joinCheckbox(v any) string {
	switch t := v.(type) {
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			s := strings.TrimSpace(util.ToString(item))
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	case []string:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			s := strings.TrimSpace(item)
			if s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		return util.ToString(v)
	}
}

func isBlankWrite(v any) bool {
	switch t := v.(type) {
	case []any:
		return len(t) == 0
	case []string:
		return len(t) == 0
	default:
		return strings.TrimSpace(util.ToString(v)) == ""
	}
}

func dictLabel(sp *spec, name string, v any) string {
	if bootstrap.DB == nil {
		return ""
	}
	var dictType string
	for _, col := range sp.cols {
		if col.ColumnName == name && col.DictType != "" {
			dictType = col.DictType
			break
		}
	}
	if dictType == "" {
		return ""
	}
	val := strings.TrimSpace(util.ToString(v))
	if val == "" {
		return ""
	}
	var row model.DictData
	q := bootstrap.DB.Where("type_value = ? AND value = ?", dictType, val).Where("delete_time IS NULL")
	if q.First(&row).Error != nil || row.Name == "" {
		return ""
	}
	return row.Name
}

func attachRelations(c *gin.Context, sp *spec, rows []map[string]any) {
	if len(rows) == 0 || len(sp.rels) == 0 || bootstrap.DB == nil {
		return
	}
	db := session(c)
	for _, rel := range sp.rels {
		ids := uniqueIDs(rows, rel.LocalKey)
		if len(ids) == 0 {
			if strings.EqualFold(rel.Type, "has_many") {
				attachHasMany(rows, nil, rel)
			}
			continue
		}
		relTable := rel.Table
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			relTable = tenantdb.Table(c, rel.Table)
		}
		var related []map[string]any
		q := db.Table(relTable).Where(rel.ForeignKey+" IN ?", ids)
		if tid := ctxutil.Get(c).TenantID; tid > 0 {
			if tableHasColumn(db, relTable, "tenant_id") {
				q = q.Where("tenant_id = ?", tid)
			}
		}
		if tableHasColumn(db, relTable, "delete_time") {
			q = q.Where("delete_time IS NULL")
		}
		if q.Find(&related).Error != nil {
			continue
		}
		for _, item := range related {
			normalizeMap(item)
		}
		if strings.EqualFold(rel.Type, "has_many") {
			attachHasMany(rows, related, rel)
			continue
		}
		index := map[string]map[string]any{}
		for _, item := range related {
			index[util.ToString(item[rel.ForeignKey])] = item
		}
		label := rel.Label
		if label == "" {
			label = pickLabel(related)
		}
		for _, row := range rows {
			item := index[util.ToString(row[rel.LocalKey])]
			if item == nil {
				continue
			}
			row[rel.Name] = item
			if label != "" {
				row[rel.Name+"_name"] = item[label]
			}
		}
	}
}

func attachHasMany(rows, related []map[string]any, rel relSpec) {
	grouped := map[string][]map[string]any{}
	for _, item := range related {
		k := util.ToString(item[rel.ForeignKey])
		grouped[k] = append(grouped[k], item)
	}
	for _, row := range rows {
		k := util.ToString(row[rel.LocalKey])
		if list := grouped[k]; list != nil {
			row[rel.Name] = list
			continue
		}
		row[rel.Name] = []map[string]any{}
	}
}

func tableHasColumn(db *gorm.DB, table, col string) bool {
	if db == nil || table == "" || col == "" {
		return false
	}
	var n int64
	if db.Raw("SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ? AND COLUMN_NAME = ?", table, col).Scan(&n).Error != nil {
		return false
	}
	return n > 0
}

func uniqueIDs(rows []map[string]any, key string) []any {
	seen := map[string]bool{}
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		v := util.ToString(row[key])
		if v == "" || v == "0" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, row[key])
	}
	return out
}

func pickLabel(rows []map[string]any) string {
	if len(rows) == 0 {
		return ""
	}
	for _, cand := range []string{"nickname", "name", "title", "label"} {
		if _, ok := rows[0][cand]; ok {
			return cand
		}
	}
	return ""
}

func normalizeMap(m map[string]any) {
	for k, v := range m {
		if b, ok := v.([]byte); ok {
			m[k] = string(b)
		}
	}
}

func treeCycleMsg(c *gin.Context, sp *spec, id uint, data map[string]any) string {
	if !sp.tree || !validIdent(sp.treePID) {
		return ""
	}
	raw, ok := data[sp.treePID]
	if !ok {
		return ""
	}
	pid := uint(util.ToInt(raw))
	if pid == 0 {
		return ""
	}
	if pid == id {
		return "上级不能选择自己"
	}
	if isTreeAncestor(session(c), sp, pid, id) {
		return "上级不能选择自己的子级"
	}
	return ""
}

func isTreeAncestor(db *gorm.DB, sp *spec, start, forbid uint) bool {
	if db == nil || start == 0 || forbid == 0 {
		return false
	}
	seen := map[uint]bool{}
	cur := start
	for i := 0; i < 64 && cur > 0 && !seen[cur]; i++ {
		if cur == forbid {
			return true
		}
		seen[cur] = true
		row := map[string]any{}
		q := db.Table(sp.table.Name).Select(sp.pk, sp.treePID).Where(sp.pk+" = ?", cur)
		if sp.softDelete {
			q = q.Where(sp.deleteCol + " IS NULL")
		}
		if q.Take(&row).Error != nil {
			return false
		}
		cur = uint(util.ToInt(row[sp.treePID]))
	}
	return false
}
