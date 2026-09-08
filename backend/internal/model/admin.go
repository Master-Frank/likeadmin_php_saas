package model

type Admin struct {
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
	CreateTime      int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime      *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime      *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (Admin) TableName() string { return T("admin") }

type AdminSession struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	AdminID    uint   `gorm:"column:admin_id" json:"admin_id"`
	Terminal   int    `gorm:"column:terminal" json:"terminal"`
	Token      string `gorm:"column:token" json:"token"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	ExpireTime int64  `gorm:"column:expire_time" json:"expire_time"`
}

func (AdminSession) TableName() string { return T("admin_session") }

type AdminRole struct {
	AdminID uint `gorm:"column:admin_id"`
	RoleID  uint `gorm:"column:role_id"`
}

func (AdminRole) TableName() string { return T("admin_role") }

type AdminDept struct {
	AdminID uint `gorm:"column:admin_id"`
	DeptID  uint `gorm:"column:dept_id"`
}

func (AdminDept) TableName() string { return T("admin_dept") }

type AdminJobs struct {
	AdminID uint `gorm:"column:admin_id"`
	JobsID  uint `gorm:"column:jobs_id"`
}

func (AdminJobs) TableName() string { return T("admin_jobs") }

type SystemRole struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Desc       string `gorm:"column:desc" json:"desc"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (SystemRole) TableName() string { return T("system_role") }

type SystemRoleMenu struct {
	RoleID uint `gorm:"column:role_id"`
	MenuID uint `gorm:"column:menu_id"`
}

func (SystemRoleMenu) TableName() string { return T("system_role_menu") }

type SystemMenu struct {
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
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
}

func (SystemMenu) TableName() string { return T("system_menu") }
