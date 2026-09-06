package generator

import (
	"strconv"
)

func (c *ctx) searchView() string {
	var content string
	for _, col := range c.cols {
		if !phpTruthy(col.Query) || phpTruthy(col.Pk) {
			continue
		}
		searchType := col.ViewType
		if searchType == "radio" {
			searchType = "select"
		}
		if !stubExists("vue/search_item/" + searchType) {
			continue
		}
		content += c.replace(readStub("vue/search_item/"+searchType), []string{
			"{COLUMN_COMMENT}", col.Comment,
			"{COLUMN_NAME}", col.Name,
			"{DICT_TYPE}", col.DictType,
		}) + "\n"
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "                ")
}

func (c *ctx) listsView() string {
	var content string
	for _, col := range c.cols {
		if !phpTruthy(col.Lists) {
			continue
		}
		stub := "vue/table_item/default"
		if col.ViewType == "imageSelect" {
			stub = "vue/table_item/image"
		}
		if col.ViewType == "select" || col.ViewType == "radio" || col.ViewType == "checkbox" {
			stub = "vue/table_item/options"
		}
		if col.Type == "int" && col.ViewType == "datetime" {
			stub = "vue/table_item/datetime"
		}
		if !stubExists(stub) {
			continue
		}
		content += c.replace(readStub(stub), []string{
			"{COLUMN_COMMENT}", col.Comment,
			"{COLUMN_NAME}", col.Name,
			"{DICT_TYPE}", col.DictType,
		}) + "\n"
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "                    ")
}

func (c *ctx) queryParams() string {
	var content string
	queryDate := false
	for _, col := range c.cols {
		if !phpTruthy(col.Query) || phpTruthy(col.Pk) {
			continue
		}
		content += col.Name + ": '',\n"
		if col.QueryType == "between" && col.ViewType == "datetime" {
			queryDate = true
		}
	}
	if queryDate {
		content += "start_time: '',\n"
		content += "end_time: '',\n"
	}
	content = phpSubstr(content, 0, -2)
	return setBlankSpace(content, "    ")
}

func (c *ctx) indexDictData() string {
	var content string
	exist := map[string]bool{}
	for _, col := range c.cols {
		if col.DictType == "" || phpTruthy(col.Pk) || exist[col.DictType] {
			continue
		}
		content += col.DictType + ","
		exist[col.DictType] = true
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "")
}

func (c *ctx) editDictData() string {
	var content string
	exist := map[string]bool{}
	for _, col := range c.cols {
		if col.DictType == "" || phpTruthy(col.Pk) || exist[col.DictType] {
			continue
		}
		content += col.DictType + ": [],\n"
		exist[col.DictType] = true
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "    ")
}

func (c *ctx) dictDataAPI() string {
	var content string
	exist := map[string]bool{}
	for _, col := range c.cols {
		if col.DictType == "" || phpTruthy(col.Pk) || exist[col.DictType] {
			continue
		}
		if !stubExists("vue/other_item/dictDataApi") {
			continue
		}
		content += c.replace(readStub("vue/other_item/dictDataApi"), []string{
			"{UPPER_CAMEL_NAME}", c.upperCamel(),
			"{DICT_TYPE}", col.DictType,
		}) + "\n"
		exist[col.DictType] = true
	}
	return phpSubstr(content, 0, -1)
}

func (c *ctx) formView() string {
	var content string
	for _, col := range c.cols {
		if !phpTruthy(col.Insert) || !phpTruthy(col.Update) || phpTruthy(col.Pk) {
			continue
		}
		need := []string{
			"{COLUMN_COMMENT}", col.Comment,
			"{COLUMN_NAME}", col.Name,
			"{DICT_TYPE}", col.DictType,
		}
		viewType := col.ViewType
		if c.isTree() && col.Name == c.treePID {
			viewType = "treeSelect"
			need = append(need, "{TREE_ID}", c.treeID, "{TREE_NAME}", c.treeName)
		}
		if !stubExists("vue/form_item/" + viewType) {
			continue
		}
		if col.ViewType == "radio" || col.ViewType == "select" {
			itemValue := "item.value"
			switch col.Type {
			case "tinyint", "smallint", "mediumint", "int", "integer", "bigint":
				itemValue = "parseInt(item.value)"
			}
			need = append(need, "{ITEM_VALUE}", itemValue)
		}
		content += c.replace(readStub("vue/form_item/"+viewType), need) + "\n"
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "                ")
}

func (c *ctx) formData() string {
	var content string
	exist := map[string]bool{}
	for _, col := range c.cols {
		if !phpTruthy(col.Insert) || !phpTruthy(col.Update) || phpTruthy(col.Pk) || exist[col.Name] {
			continue
		}
		if col.ViewType == "checkbox" {
			content += col.Name + ": [],\n"
		} else {
			content += col.Name + ": '',\n"
		}
		exist[col.Name] = true
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "    ")
}

func (c *ctx) formValidate() string {
	var content string
	exist := map[string]bool{}
	spec := map[string]bool{"input": true, "textarea": true, "editor": true}
	for _, col := range c.cols {
		if !phpTruthy(col.Required) || phpTruthy(col.Pk) || exist[col.Name] {
			continue
		}
		msg := "请选择"
		if spec[col.ViewType] {
			msg = "请输入"
		}
		msg += col.Comment
		if !stubExists("vue/other_item/formValidate") {
			continue
		}
		content += c.replace(readStub("vue/other_item/formValidate"), []string{
			"{COLUMN_NAME}", col.Name,
			"{VALIDATE_MSG}", msg,
		}) + ",\n"
		exist[col.Name] = true
	}
	return phpSubstr(content, 0, -2)
}

func (c *ctx) checkboxJoin() string {
	var content string
	for _, col := range c.cols {
		if col.ViewType == "" || phpTruthy(col.Pk) || col.ViewType != "checkbox" {
			continue
		}
		content += col.Name + ": formData." + col.Name + ".join(\",\")\n"
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return content
}

func (c *ctx) checkboxSplit() string {
	var content string
	for _, col := range c.cols {
		if col.ViewType == "" || phpTruthy(col.Pk) || col.ViewType != "checkbox" {
			continue
		}
		content += "//@ts-ignore\n"
		content += "data." + col.Name + " && (formData." + col.Name + " = String(data." + col.Name + ").split(\",\"))\n"
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "    ")
}

func (c *ctx) formDate() string {
	var content string
	for _, col := range c.cols {
		if col.ViewType == "" || phpTruthy(col.Pk) {
			continue
		}
		if col.ViewType != "datetime" || col.Type != "int" {
			continue
		}
		content += "//@ts-ignore\n"
		content += "formData." + col.Name + " = timeFormat(formData." + col.Name + ",'yyyy-mm-dd hh:MM:ss') \n"
	}
	if content != "" {
		content = phpSubstr(content, 0, -1)
	}
	return setBlankSpace(content, "    ")
}

func (c *ctx) treeConst() string {
	if !c.isTree() {
		return ""
	}
	return readStub("vue/other_item/editTreeConst")
}

func (c *ctx) treeLists() string {
	if !c.isTree() {
		return ""
	}
	return c.replace(readStub("vue/other_item/editTreeLists"), []string{
		"{TREE_ID}", c.treeID,
		"{TREE_NAME}", c.treeName,
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
	})
}

func (c *ctx) importLists() string {
	if !c.isTree() {
		return ""
	}
	return setBlankSpace("api"+c.upperCamel()+"Lists,", " ")
}

func (c *ctx) genVueAPI() File {
	content := c.replace(readStub("vue/api"), []string{
		"{COMMENT}", c.comment,
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{ROUTE}", c.vueRoute(),
	})
	return File{
		Name:    c.lowerTable() + ".ts",
		Type:    "ts",
		Content: content,
		RelPath: phpDir("vue/src/api", c.lowerTable()+".ts"),
	}
}

func (c *ctx) genVueIndex() File {
	need := []string{
		"{SEARCH_VIEW}", c.searchView(),
		"{LISTS_VIEW}", c.listsView(),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{QUERY_PARAMS}", c.queryParams(),
		"{DICT_DATA}", c.indexDictData(),
		"{PK}", c.pk(),
		"{API_DIR}", c.tableName,
		"{PERMS_ADD}", c.perms("add"),
		"{PERMS_EDIT}", c.perms("edit"),
		"{PERMS_DELETE}", c.perms("delete"),
		"{SETUP_NAME}", c.lowerCamel(),
	}
	stub := "vue/index"
	if c.isTree() {
		need = append(need, "{TREE_ID}", c.treeID, "{TREE_PID}", c.treePID)
		stub = "vue/index-tree"
	}
	return File{
		Name:    "index.vue",
		Type:    "vue",
		Content: c.replace(readStub(stub), need),
		RelPath: phpDir("vue/src/views", c.lowerTable(), "index.vue"),
	}
}

func (c *ctx) genVueEdit() File {
	content := c.replace(readStub("vue/edit"), []string{
		"{FORM_VIEW}", c.formView(),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{DICT_DATA}", c.editDictData(),
		"{DICT_DATA_API}", c.dictDataAPI(),
		"{FORM_DATA}", c.formData(),
		"{FORM_VALIDATE}", c.formValidate(),
		"{TABLE_COMMENT}", c.comment,
		"{PK}", c.pk(),
		"{API_DIR}", c.tableName,
		"{CHECKBOX_JOIN}", c.checkboxJoin(),
		"{CHECKBOX_SPLIT}", c.checkboxSplit(),
		"{FORM_DATE}", c.formDate(),
		"{SETUP_NAME}", c.lowerCamel(),
		"{IMPORT_LISTS}", c.importLists(),
		"{TREE_CONST}", c.treeConst(),
		"{GET_TREE_LISTS}", c.treeLists(),
	})
	return File{
		Name:    "edit.vue",
		Type:    "vue",
		Content: content,
		RelPath: phpDir("vue/src/views", c.tableName, "edit.vue"),
	}
}

func (c *ctx) genSQL() File {
	content := c.replace(readStub("sql/sql"), []string{
		"{MENU_TABLE}", c.menuTable(),
		"{PARTNER_ID}", itoa(c.menuPid),
		"{LISTS_NAME}", c.menuName,
		"{PERMS_NAME}", c.permsName(),
		"{PATHS_NAME}", c.lowerTable(),
		"{COMPONENT_NAME}", c.lowerTable(),
		"{CREATE_TIME}", itoa64(c.unixNow),
		"{UPDATE_TIME}", itoa64(c.unixNow),
	})
	return File{
		Name:    "menu.sql",
		Type:    "sql",
		Content: content,
		RelPath: "sql/menu.sql",
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
