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
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	Name         string `gorm:"column:name" json:"name"`
	Type         string `gorm:"column:type" json:"type"`
	Status       int    `gorm:"column:status" json:"status"`
	Remark       string `gorm:"column:remark" json:"remark"`
	CreateTime   int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime   *int64 `gorm:"column:delete_time" json:"delete_time"`
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
