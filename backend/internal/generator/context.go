package generator

import (
	"strings"
	"time"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
)

const (
	GenerateTypeZip    = 0
	GenerateTypeModule = 1
	TemplateTypeSingle = 0
	TemplateTypeTree   = 1
	DeleteTrue         = 0
	DeleteSoft         = 1
	GenSelf            = 0
	GenAuto            = 1
)

// File is one generated artifact in PHP fileInfo() / zip shape.
type File struct {
	Name    string // basename for preview
	Type    string // php / ts / vue / sql
	Content string
	RelPath string // path under runtime/generate (no generate/ prefix)
}

func (f File) Preview() map[string]any {
	return map[string]any{
		"name":    f.Name,
		"type":    f.Type,
		"content": f.Content,
	}
}

func (f File) ZipName() string {
	return "generate/" + strings.ReplaceAll(f.RelPath, "\\", "/")
}

type column struct {
	Name      string
	Comment   string
	Type      string
	Required  int
	Pk        int
	Insert    int
	Update    int
	Lists     int
	Query     int
	QueryType string
	ViewType  string
	DictType  string
}

type ctx struct {
	table        string
	tableName    string
	comment      string
	classComment string
	author       string
	module       string
	classDir     string
	templateType int
	generateType int
	noteDate     string
	unixNow      int64
	cols         []column
	menuPid      int
	menuType     int
	menuName     string
	deleteType   int
	deleteName   string
	treeID       string
	treePID      string
	treeName     string
	relations    []map[string]string
}

func newCtx(t model.GenerateTable, cols []model.GenerateColumn, now time.Time) *ctx {
	menu := util.DecodeJSONMap(t.Menu)
	del := util.DecodeJSONMap(t.Delete)
	tree := util.DecodeJSONMap(t.Tree)
	relRaw := util.DecodeJSON(t.Relations)
	rels := make([]map[string]string, 0)
	if arr, ok := relRaw.([]any); ok {
		for _, item := range arr {
			m, _ := item.(map[string]any)
			if m == nil {
				continue
			}
			rels = append(rels, map[string]string{
				"name":        util.ToString(m["name"]),
				"model":       util.ToString(m["model"]),
				"type":        util.ToString(m["type"]),
				"local_key":   util.ToString(m["local_key"]),
				"foreign_key": util.ToString(m["foreign_key"]),
			})
		}
	}
	module := strings.ToLower(t.ModuleName)
	if module == "" {
		module = "platform"
	}
	author := t.Author
	if author == "" {
		author = "likeadmin"
	}
	menuName := util.ToString(menu["name"])
	if menuName == "" {
		menuName = t.TableComment
	}
	delName := util.ToString(del["name"])
	if delName == "" {
		delName = "delete_time"
	}
	out := &ctx{
		table:        t.Name,
		tableName:    NoPrefix(t.Name),
		comment:      t.TableComment,
		classComment: t.ClassComment,
		author:       author,
		module:       module,
		classDir:     strings.Trim(t.ClassDir, "\\/"),
		templateType: t.TemplateType,
		generateType: t.GenerateType,
		noteDate:     now.Format("2006/01/02 15:04"),
		unixNow:      now.Unix(),
		menuPid:      util.ToInt(menu["pid"]),
		menuType:     util.ToInt(menu["type"]),
		menuName:     menuName,
		deleteType:   util.ToInt(del["type"]),
		deleteName:   delName,
		treeID:       util.ToString(tree["tree_id"]),
		treePID:      util.ToString(tree["tree_pid"]),
		treeName:     util.ToString(tree["tree_name"]),
		relations:    rels,
	}
	for _, col := range cols {
		out.cols = append(out.cols, column{
			Name: col.ColumnName, Comment: col.ColumnComment, Type: col.ColumnType,
			Required: col.IsRequired, Pk: col.IsPk, Insert: col.IsInsert, Update: col.IsUpdate,
			Lists: col.IsLists, Query: col.IsQuery, QueryType: col.QueryType,
			ViewType: col.ViewType, DictType: col.DictType,
		})
	}
	return out
}

func (c *ctx) upperCamel() string { return Studly(c.tableName) }
func (c *ctx) lowerCamel() string { return Camel(c.tableName) }
func (c *ctx) lowerTable() string { return Lower(c.tableName) }
func (c *ctx) isTree() bool       { return c.templateType == TemplateTypeTree }

func (c *ctx) pk() string {
	for _, col := range c.cols {
		if phpTruthy(col.Pk) {
			return col.Name
		}
	}
	return "id"
}

func (c *ctx) notes() string {
	if c.classComment != "" {
		return c.classComment
	}
	return ""
}

func (c *ctx) classCommentWith(suffix string) string {
	if c.classComment != "" {
		return c.classComment + suffix
	}
	return c.upperCamel() + suffix
}

func (c *ctx) packageNS() string {
	if c.classDir != "" {
		return "\\" + c.classDir
	}
	return ""
}

func (c *ctx) listsPackage() string {
	if c.classDir != "" {
		return c.classDir
	}
	return ""
}

func (c *ctx) phpNS(kind string) string {
	if c.classDir != "" {
		return "namespace app\\" + c.module + "\\" + kind + "\\" + c.classDir + ";"
	}
	return "namespace app\\" + c.module + "\\" + kind + ";"
}

func (c *ctx) modelNS() string {
	if c.classDir != "" {
		return "namespace app\\common\\model\\" + c.classDir + ";"
	}
	return "namespace app\\common\\model;"
}

func (c *ctx) modelUse() string {
	if c.classDir != "" {
		return "use app\\common\\model\\" + c.classDir + "\\" + c.upperCamel() + ";"
	}
	return "use app\\common\\model\\" + c.upperCamel() + ";"
}

func (c *ctx) permsName() string {
	// PHP SqlGenerator::getPermsNameContent keeps classDir case.
	if c.classDir != "" {
		return c.classDir + "." + Lower(c.tableName)
	}
	return Lower(c.tableName)
}

func (c *ctx) vueRoute() string {
	// PHP VueApiGenerator::getRouteContent lowercases the whole route.
	return Lower(c.permsName())
}

func (c *ctx) perms(kind string) string {
	prefix := ""
	if c.classDir != "" {
		prefix = c.classDir + "."
	}
	return strings.TrimSpace(prefix + c.lowerTable() + "/" + kind)
}

func (c *ctx) menuTable() string {
	return config.Prefix() + "system_menu"
}

func (c *ctx) replace(tpl string, pairs []string) string {
	return strings.NewReplacer(pairs...).Replace(tpl)
}

func phpDir(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return strings.Join(out, "/")
}
