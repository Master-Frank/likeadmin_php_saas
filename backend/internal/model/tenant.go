package model

type Tenant struct {
	ID                uint   `gorm:"column:id;primaryKey" json:"id"`
	SN                string `gorm:"column:sn" json:"sn"`
	Name              string `gorm:"column:name" json:"name"`
	Avatar            string `gorm:"column:avatar" json:"avatar"`
	Tel               string `gorm:"column:tel" json:"tel"`
	Disable           int    `gorm:"column:disable" json:"disable"`
	Tactics           int    `gorm:"column:tactics" json:"tactics"`
	DomainAlias       string `gorm:"column:domain_alias" json:"domain_alias"`
	DomainAliasEnable int    `gorm:"column:domain_alias_enable" json:"domain_alias_enable"`
	Notes             string `gorm:"column:notes" json:"notes"`
	CreateTime        int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime        *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime        *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (Tenant) TableName() string { return T("tenant") }

type TenantAdmin struct {
	ID              uint   `gorm:"column:id;primaryKey" json:"id"`
	Root            int    `gorm:"column:root" json:"root"`
	Name            string `gorm:"column:name" json:"name"`
	Avatar          string `gorm:"column:avatar" json:"avatar"`
	Account         string `gorm:"column:account" json:"account"`
	Password        string `gorm:"column:password" json:"-"`
	LoginTime       *int64 `gorm:"column:login_time" json:"login_time"`
	LoginIP         string `gorm:"column:login_ip" json:"login_ip"`
	MultipointLogin int    `gorm:"column:multipoint_login" json:"multipoint_login"`
	Disable         int    `gorm:"column:disable" json:"disable"`
	TenantID        uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime      int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime      *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime      *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantAdmin) TableName() string { return T("tenant_admin") }

type TenantAdminSession struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	AdminID    uint   `gorm:"column:admin_id" json:"admin_id"`
	Terminal   int    `gorm:"column:terminal" json:"terminal"`
	Token      string `gorm:"column:token" json:"token"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	ExpireTime int64  `gorm:"column:expire_time" json:"expire_time"`
}

func (TenantAdminSession) TableName() string { return T("tenant_admin_session") }

type TenantAdminRole struct {
	AdminID uint `gorm:"column:admin_id"`
	RoleID  uint `gorm:"column:role_id"`
}

func (TenantAdminRole) TableName() string { return T("tenant_admin_role") }

type TenantAdminDept struct {
	AdminID uint `gorm:"column:admin_id"`
	DeptID  uint `gorm:"column:dept_id"`
}

func (TenantAdminDept) TableName() string { return T("tenant_admin_dept") }

type TenantAdminJobs struct {
	AdminID uint `gorm:"column:admin_id"`
	JobsID  uint `gorm:"column:jobs_id"`
}

func (TenantAdminJobs) TableName() string { return T("tenant_admin_jobs") }

type TenantSystemRole struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Desc       string `gorm:"column:desc" json:"desc"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantSystemRole) TableName() string { return T("tenant_system_role") }

type TenantSystemRoleMenu struct {
	RoleID uint `gorm:"column:role_id"`
	MenuID uint `gorm:"column:menu_id"`
}

func (TenantSystemRoleMenu) TableName() string { return T("tenant_system_role_menu") }

type TenantSystemMenu struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Pid        uint   `gorm:"column:pid" json:"pid"`
	Type       string `gorm:"column:type" json:"type"`
	Name       string `gorm:"column:name" json:"name"`
	Icon       string `gorm:"column:icon" json:"icon"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	Perms      string `gorm:"column:perms" json:"perms"`
	Paths      string `gorm:"column:paths" json:"paths"`
	Component  string `gorm:"column:component" json:"component"`
	Selected   string `gorm:"column:selected" json:"selected"`
	Params     string `gorm:"column:params" json:"params"`
	IsCache    int    `gorm:"column:is_cache" json:"is_cache"`
	IsShow     int    `gorm:"column:is_show" json:"is_show"`
	IsDisable  int    `gorm:"column:is_disable" json:"is_disable"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

func (TenantSystemMenu) TableName() string { return T("tenant_system_menu") }
