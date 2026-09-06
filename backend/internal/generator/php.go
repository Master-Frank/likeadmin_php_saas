package generator

import (
	"strings"
)

func (c *ctx) controllerUse() string {
	var tpl string
	if c.module == "platformapi" || c.module == "tenantapi" {
		tpl = "use app\\" + c.module + "\\controller\\BaseAdminController;\n"
	} else {
		tpl = "use app\\common\\controller\\BaseLikeAdminController;\n"
	}
	name := c.upperCamel()
	if c.classDir != "" {
		tpl += "use app\\" + c.module + "\\lists\\" + c.classDir + "\\" + name + "Lists;\n" +
			"use app\\" + c.module + "\\logic\\" + c.classDir + "\\" + name + "Logic;\n" +
			"use app\\" + c.module + "\\validate\\" + c.classDir + "\\" + name + "Validate;"
	} else {
		tpl += "use app\\" + c.module + "\\lists\\" + name + "Lists;\n" +
			"use app\\" + c.module + "\\logic\\" + name + "Logic;\n" +
			"use app\\" + c.module + "\\validate\\" + name + "Validate;"
	}
	return tpl
}

func (c *ctx) extendsController() string {
	// PHP ControllerGenerator::getExtendsControllerContent uses
	// `!= platformapi || != tenantapi`, which is always true, so the
	// generated class always extends BaseLikeAdminController.
	return "BaseLikeAdminController"
}

func (c *ctx) listsUse() string {
	var tpl string
	if c.module == "api" {
		tpl = "use app\\common\\lists\\BaseDataLists;\n"
	} else {
		tpl = "use app\\" + c.module + "\\lists\\BaseAdminDataLists;\n"
	}
	tpl += c.modelUse()
	return tpl
}

func (c *ctx) extendsLists() string {
	if c.module == "api" {
		return "BaseAdminDataLists"
	}
	return "BaseDataLists"
}

func (c *ctx) queryCondition() string {
	seen := map[string]bool{}
	var queryNames []string
	for _, col := range c.cols {
		if col.QueryType == "" || seen[col.QueryType] {
			continue
		}
		seen[col.QueryType] = true
		queryNames = append(queryNames, col.QueryType)
	}
	spec := map[string]bool{"between": true, "like": true}
	var cond string
	for _, queryName := range queryNames {
		var columnValue string
		for _, col := range c.cols {
			if col.QueryType == "" || phpTruthy(col.Pk) {
				continue
			}
			if queryName == col.QueryType && phpTruthy(col.Query) && !spec[queryName] {
				columnValue += "'" + col.Name + "', "
			}
		}
		if columnValue != "" {
			columnValue = phpSubstr(columnValue, 0, -2)
			cond += "'" + queryName + "' => [" + strings.TrimSpace(columnValue) + "],\n"
		}
	}
	var likeColumn, betweenColumn, betweenTimeColumn string
	for _, item := range c.cols {
		if !phpTruthy(item.Query) {
			continue
		}
		if item.QueryType == "like" {
			likeColumn += "'" + item.Name + "', "
			continue
		}
		if item.QueryType == "between" {
			if item.ViewType == "datetime" {
				betweenTimeColumn += "'" + item.Name + "', "
			} else {
				betweenColumn += "'" + item.Name + "', "
			}
		}
	}
	if likeColumn != "" {
		likeColumn = phpSubstr(likeColumn, 0, -2)
		cond += "'%like%' => [" + strings.TrimSpace(likeColumn) + "],\n"
	}
	if betweenColumn != "" {
		betweenColumn = phpSubstr(betweenColumn, 0, -2)
		cond += "'between' => [" + strings.TrimSpace(betweenColumn) + "],\n"
	}
	if betweenTimeColumn != "" {
		betweenTimeColumn = phpSubstr(betweenTimeColumn, 0, -2)
		cond += "'between_time' => [" + strings.TrimSpace(betweenTimeColumn) + "],\n"
	}
	content := phpSubstr(cond, 0, -1)
	return setBlankSpace(content, "            ")
}

func (c *ctx) fieldData() string {
	pk := c.pk()
	content := "'" + pk + "', "
	exist := map[string]bool{pk: true}
	for _, col := range c.cols {
		if phpTruthy(col.Lists) && !exist[col.Name] {
			content += "'" + col.Name + "', "
			exist[col.Name] = true
		}
		if c.isTree() && !exist[col.Name] && (col.Name == c.treeID || col.Name == c.treePID) {
			content += "'" + col.Name + "', "
			exist[col.Name] = true
		}
	}
	return phpSubstr(content, 0, -2)
}

func (c *ctx) addEditColumn(col column) string {
	if col.Type == "int" && col.ViewType == "datetime" {
		return "'" + col.Name + "' => strtotime($params['" + col.Name + "']),\n"
	}
	return "'" + col.Name + "' => $params['" + col.Name + "'],\n"
}

func (c *ctx) createData() string {
	var content string
	for _, col := range c.cols {
		if !phpTruthy(col.Insert) {
			continue
		}
		content += c.addEditColumn(col)
	}
	if content == "" {
		return content
	}
	content = phpSubstr(content, 0, -2)
	return setBlankSpace(content, "                ")
}

func (c *ctx) updateData() string {
	var columnContent string
	for _, col := range c.cols {
		if !phpTruthy(col.Update) {
			continue
		}
		columnContent += c.addEditColumn(col)
	}
	if columnContent == "" {
		return columnContent
	}
	columnContent = phpSubstr(columnContent, 0, -2)
	return setBlankSpace(columnContent, "                ")
}

func (c *ctx) ruleContent() string {
	content := "'" + c.pk() + "' => 'require',\n"
	for _, col := range c.cols {
		if col.Required == 1 {
			content += "'" + col.Name + "' => 'require',\n"
		}
	}
	content = phpSubstr(content, 0, -1)
	return setBlankSpace(content, "        ")
}

func (c *ctx) addParams() string {
	var content string
	for _, col := range c.cols {
		if col.Required == 1 && col.Name != c.pk() {
			content += "'" + col.Name + "',"
		}
	}
	content = phpSubstr(content, 0, -1)
	if content != "" {
		content = "return $this->only([" + content + "]);"
	} else {
		content = "return $this->remove('" + c.pk() + "', true);"
	}
	return setBlankSpace(content, "")
}

func (c *ctx) editParams() string {
	content := "'" + c.pk() + "',"
	for _, col := range c.cols {
		if col.Required == 1 {
			content += "'" + col.Name + "',"
		}
	}
	content = phpSubstr(content, 0, -1)
	if content != "" {
		content = "return $this->only([" + content + "]);"
	}
	return setBlankSpace(content, "")
}

func (c *ctx) fieldDesc() string {
	content := "'" + c.pk() + "' => '" + c.pk() + "',\n"
	for _, col := range c.cols {
		if col.Required == 1 {
			comment := col.Comment
			if comment == "" {
				comment = col.Name
			}
			content += "'" + col.Name + "' => '" + comment + "',\n"
		}
	}
	content = phpSubstr(content, 0, -1)
	return setBlankSpace(content, "        ")
}

func (c *ctx) relationModel() string {
	var tpl string
	for _, rel := range c.relations {
		if rel["name"] == "" || rel["model"] == "" {
			continue
		}
		stub := readStub("php/model/" + rel["type"])
		if stub == "" {
			continue
		}
		tpl += c.replace(stub, []string{
			"{RELATION_NAME}", rel["name"],
			"{AUTHOR}", c.author,
			"{DATE}", c.noteDate,
			"{RELATION_MODEL}", rel["model"],
			"{FOREIGN_KEY}", rel["foreign_key"],
			"{LOCAL_KEY}", rel["local_key"],
		}) + "\n"
	}
	return tpl
}

func (c *ctx) genController() File {
	content := c.replace(readStub("php/controller"), []string{
		"{NAMESPACE}", c.phpNS("controller"),
		"{USE}", c.controllerUse(),
		"{CLASS_COMMENT}", c.classCommentWith("控制器"),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{MODULE_NAME}", c.module,
		"{PACKAGE_NAME}", c.packageNS(),
		"{EXTENDS_CONTROLLER}", c.extendsController(),
		"{NOTES}", c.notes(),
		"{AUTHOR}", c.author,
		"{DATE}", c.noteDate,
	})
	return File{
		Name:    c.upperCamel() + "Controller.php",
		Type:    "php",
		Content: content,
		RelPath: phpDir("php/app", c.module, "controller", c.classDir, c.upperCamel()+"Controller.php"),
	}
}

func (c *ctx) genLists() File {
	need := []string{
		"{NAMESPACE}", c.phpNS("lists"),
		"{USE}", c.listsUse(),
		"{CLASS_COMMENT}", c.classCommentWith("列表"),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{MODULE_NAME}", c.module,
		"{PACKAGE_NAME}", c.listsPackage(),
		"{EXTENDS_LISTS}", c.extendsLists(),
		"{PK}", c.pk(),
		"{QUERY_CONDITION}", c.queryCondition(),
		"{FIELD_DATA}", c.fieldData(),
		"{NOTES}", c.notes(),
		"{AUTHOR}", c.author,
		"{DATE}", c.noteDate,
	}
	stub := "php/lists"
	if c.isTree() {
		need = append(need, "{TREE_ID}", c.treeID, "{TREE_PID}", c.treePID)
		stub = "php/tree_lists"
	}
	return File{
		Name:    c.upperCamel() + "Lists.php",
		Type:    "php",
		Content: c.replace(readStub(stub), need),
		RelPath: phpDir("php/app", c.module, "lists", c.classDir, c.upperCamel()+"Lists.php"),
	}
}

func (c *ctx) genModel() File {
	use, delUse, delTime := "", "", ""
	if phpTruthy(c.deleteType) {
		use = "use think\\model\\concern\\SoftDelete;"
		delUse = "use SoftDelete;"
		delTime = "protected $deleteTime = '" + c.deleteName + "';"
	}
	content := c.replace(readStub("php/model"), []string{
		"{NAMESPACE}", c.modelNS(),
		"{CLASS_COMMENT}", c.classCommentWith("模型"),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{PACKAGE_NAME}", c.packageNS(),
		"{TABLE_NAME}", c.tableName,
		"{USE}", use,
		"{DELETE_USE}", delUse,
		"{DELETE_TIME}", delTime,
		"{RELATION_MODEL}", c.relationModel(),
	})
	return File{
		Name:    c.upperCamel() + ".php",
		Type:    "php",
		Content: content,
		RelPath: phpDir("php/app/common/model", c.classDir, c.upperCamel()+".php"),
	}
}

func (c *ctx) genValidate() File {
	content := c.replace(readStub("php/validate"), []string{
		"{NAMESPACE}", c.phpNS("validate"),
		"{CLASS_COMMENT}", c.classCommentWith("验证器"),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{MODULE_NAME}", c.module,
		"{PACKAGE_NAME}", c.packageNS(),
		"{PK}", c.pk(),
		"{RULE}", c.ruleContent(),
		"{NOTES}", c.notes(),
		"{AUTHOR}", c.author,
		"{DATE}", c.noteDate,
		"{ADD_PARAMS}", c.addParams(),
		"{EDIT_PARAMS}", c.editParams(),
		"{FIELD}", c.fieldDesc(),
	})
	return File{
		Name:    c.upperCamel() + "Validate.php",
		Type:    "php",
		Content: content,
		RelPath: phpDir("php/app", c.module, "validate", c.classDir, c.upperCamel()+"Validate.php"),
	}
}

func (c *ctx) genLogic() File {
	content := c.replace(readStub("php/logic"), []string{
		"{NAMESPACE}", c.phpNS("logic"),
		"{USE}", c.modelUse(),
		"{CLASS_COMMENT}", c.classCommentWith("逻辑"),
		"{UPPER_CAMEL_NAME}", c.upperCamel(),
		"{MODULE_NAME}", c.module,
		"{PACKAGE_NAME}", c.packageNS(),
		"{PK}", c.pk(),
		"{CREATE_DATA}", c.createData(),
		"{UPDATE_DATA}", c.updateData(),
		"{NOTES}", c.notes(),
		"{AUTHOR}", c.author,
		"{DATE}", c.noteDate,
	})
	return File{
		Name:    c.upperCamel() + "Logic.php",
		Type:    "php",
		Content: content,
		RelPath: phpDir("php/app", c.module, "logic", c.classDir, c.upperCamel()+"Logic.php"),
	}
}
