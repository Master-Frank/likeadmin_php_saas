package model

type ConfigRow struct {
	ID         uint   `gorm:"column:id;primaryKey"`
	Type       string `gorm:"column:type"`
	Name       string `gorm:"column:name"`
	Value      string `gorm:"column:value"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime"`
}

func (ConfigRow) TableName() string { return T("config") }

type TenantConfig struct {
	ID         uint   `gorm:"column:id;primaryKey"`
	Type       string `gorm:"column:type"`
	Name       string `gorm:"column:name"`
	Value      string `gorm:"column:value"`
	TenantID   uint   `gorm:"column:tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime"`
}

func (TenantConfig) TableName() string { return T("tenant_config") }

type DictType struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Type       string `gorm:"column:type" json:"type"`
	Status     int    `gorm:"column:status" json:"status"`
	Remark     string `gorm:"column:remark" json:"remark"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (DictType) TableName() string { return T("dict_type") }

type DictData struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Value      string `gorm:"column:value" json:"value"`
	TypeID     uint   `gorm:"column:type_id" json:"type_id"`
	TypeValue  string `gorm:"column:type_value" json:"type_value"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Status     int    `gorm:"column:status" json:"status"`
	Remark     string `gorm:"column:remark" json:"remark"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (DictData) TableName() string { return T("dict_data") }

type Dept struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Pid        uint   `gorm:"column:pid" json:"pid"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Leader     string `gorm:"column:leader" json:"leader"`
	Mobile     string `gorm:"column:mobile" json:"mobile"`
	Status     int    `gorm:"column:status" json:"status"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (Dept) TableName() string { return T("dept") }

type Jobs struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Code       string `gorm:"column:code" json:"code"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Status     int    `gorm:"column:status" json:"status"`
	Remark     string `gorm:"column:remark" json:"remark"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (Jobs) TableName() string { return T("jobs") }

type TenantDept struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Pid        uint   `gorm:"column:pid" json:"pid"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Leader     string `gorm:"column:leader" json:"leader"`
	Mobile     string `gorm:"column:mobile" json:"mobile"`
	Status     int    `gorm:"column:status" json:"status"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantDept) TableName() string { return T("tenant_dept") }

type TenantJobs struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Code       string `gorm:"column:code" json:"code"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Status     int    `gorm:"column:status" json:"status"`
	Remark     string `gorm:"column:remark" json:"remark"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantJobs) TableName() string { return T("tenant_jobs") }

type File struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Cid        uint   `gorm:"column:cid" json:"cid"`
	SourceID   uint   `gorm:"column:source_id" json:"source_id"`
	Type       int    `gorm:"column:type" json:"type"`
	Name       string `gorm:"column:name" json:"name"`
	URI        string `gorm:"column:uri" json:"uri"`
	Source     int    `gorm:"column:source" json:"source"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (File) TableName() string { return T("file") }

type FileCate struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Pid        uint   `gorm:"column:pid" json:"pid"`
	Type       int    `gorm:"column:type" json:"type"`
	Name       string `gorm:"column:name" json:"name"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (FileCate) TableName() string { return T("file_cate") }

type TenantFile struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Cid        uint   `gorm:"column:cid" json:"cid"`
	SourceID   uint   `gorm:"column:source_id" json:"source_id"`
	Type       int    `gorm:"column:type" json:"type"`
	Name       string `gorm:"column:name" json:"name"`
	URI        string `gorm:"column:uri" json:"uri"`
	Source     int    `gorm:"column:source" json:"source"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantFile) TableName() string { return T("tenant_file") }

type TenantFileCate struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Pid        uint   `gorm:"column:pid" json:"pid"`
	Type       int    `gorm:"column:type" json:"type"`
	Name       string `gorm:"column:name" json:"name"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantFileCate) TableName() string { return T("tenant_file_cate") }

type OperationLog struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	AdminID    uint   `gorm:"column:admin_id" json:"admin_id"`
	AdminName  string `gorm:"column:admin_name" json:"admin_name"`
	Action     string `gorm:"column:action" json:"action"`
	Type       string `gorm:"column:type" json:"type"`
	URL        string `gorm:"column:url" json:"url"`
	Params     string `gorm:"column:params" json:"params"`
	Result     string `gorm:"column:result" json:"result"`
	IP         string `gorm:"column:ip" json:"ip"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

func (OperationLog) TableName() string { return T("operation_log") }

type Crontab struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Type       int    `gorm:"column:type" json:"type"`
	System     int    `gorm:"column:system" json:"system"`
	Remark     string `gorm:"column:remark" json:"remark"`
	Command    string `gorm:"column:command" json:"command"`
	Params     string `gorm:"column:params" json:"params"`
	Status     int    `gorm:"column:status" json:"status"`
	Expression string `gorm:"column:expression" json:"expression"`
	Error      string `gorm:"column:error" json:"error"`
	LastTime   *int64 `gorm:"column:last_time" json:"last_time"`
	Time       string `gorm:"column:time" json:"time"`
	MaxTime    string `gorm:"column:max_time" json:"max_time"`
}

func (Crontab) TableName() string { return T("dev_crontab") }

type GenerateTable struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	Name         string `gorm:"column:table_name" json:"table_name"`
	TableComment string `gorm:"column:table_comment" json:"table_comment"`
	TemplateType int    `gorm:"column:template_type" json:"template_type"`
	Author       string `gorm:"column:author" json:"author"`
	Remark       string `gorm:"column:remark" json:"remark"`
	GenerateType int    `gorm:"column:generate_type" json:"generate_type"`
	ModuleName   string `gorm:"column:module_name" json:"module_name"`
	ClassDir     string `gorm:"column:class_dir" json:"class_dir"`
	ClassComment string `gorm:"column:class_comment" json:"class_comment"`
	AdminID      uint   `gorm:"column:admin_id" json:"admin_id"`
	Menu         string `gorm:"column:menu" json:"menu"`
	Delete       string `gorm:"column:delete" json:"delete"`
	Tree         string `gorm:"column:tree" json:"tree"`
	Relations    string `gorm:"column:relations" json:"relations"`
	CreateTime   int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (GenerateTable) TableName() string { return T("generate_table") }

type GenerateColumn struct {
	ID            uint   `gorm:"column:id;primaryKey" json:"id"`
	TableID       uint   `gorm:"column:table_id" json:"table_id"`
	ColumnName    string `gorm:"column:column_name" json:"column_name"`
	ColumnComment string `gorm:"column:column_comment" json:"column_comment"`
	ColumnType    string `gorm:"column:column_type" json:"column_type"`
	IsRequired    int    `gorm:"column:is_required" json:"is_required"`
	IsPk          int    `gorm:"column:is_pk" json:"is_pk"`
	IsInsert      int    `gorm:"column:is_insert" json:"is_insert"`
	IsUpdate      int    `gorm:"column:is_update" json:"is_update"`
	IsLists       int    `gorm:"column:is_lists" json:"is_lists"`
	IsQuery       int    `gorm:"column:is_query" json:"is_query"`
	QueryType     string `gorm:"column:query_type" json:"query_type"`
	ViewType      string `gorm:"column:view_type" json:"view_type"`
	DictType      string `gorm:"column:dict_type" json:"dict_type"`
	CreateTime    int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime    *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (GenerateColumn) TableName() string { return T("generate_column") }
