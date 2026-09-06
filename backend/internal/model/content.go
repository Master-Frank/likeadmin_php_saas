package model

type User struct {
	ID                  uint    `gorm:"column:id;primaryKey" json:"id"`
	SN                  int     `gorm:"column:sn" json:"sn"`
	Avatar              string  `gorm:"column:avatar" json:"avatar"`
	RealName            string  `gorm:"column:real_name" json:"real_name"`
	Nickname            string  `gorm:"column:nickname" json:"nickname"`
	Account             string  `gorm:"column:account" json:"account"`
	Password            string  `gorm:"column:password" json:"-"`
	Mobile              string  `gorm:"column:mobile" json:"mobile"`
	Sex                 int     `gorm:"column:sex" json:"sex"`
	Channel             int     `gorm:"column:channel" json:"channel"`
	IsDisable           int     `gorm:"column:is_disable" json:"is_disable"`
	LoginIP             string  `gorm:"column:login_ip" json:"login_ip"`
	LoginTime           *int64  `gorm:"column:login_time" json:"login_time"`
	IsNewUser           int     `gorm:"column:is_new_user" json:"is_new_user"`
	UserMoney           float64 `gorm:"column:user_money" json:"user_money"`
	TotalRechargeAmount float64 `gorm:"column:total_recharge_amount" json:"total_recharge_amount"`
	TenantID            uint    `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime          int64   `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime          *int64  `gorm:"column:update_time" json:"update_time"`
	DeleteTime          *int64  `gorm:"column:delete_time" json:"delete_time"`
}

func (User) TableName() string { return T("user") }

type UserSession struct {
	ID         uint   `gorm:"column:id;primaryKey"`
	TenantID   uint   `gorm:"column:tenant_id"`
	UserID     uint   `gorm:"column:user_id"`
	Terminal   int    `gorm:"column:terminal"`
	Token      string `gorm:"column:token"`
	UpdateTime *int64 `gorm:"column:update_time"`
	ExpireTime int64  `gorm:"column:expire_time"`
}

func (UserSession) TableName() string { return T("user_session") }

type UserAuth struct {
	ID         uint   `gorm:"column:id;primaryKey"`
	UserID     uint   `gorm:"column:user_id"`
	Openid     string `gorm:"column:openid"`
	Unionid    string `gorm:"column:unionid"`
	Terminal   int    `gorm:"column:terminal"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime"`
	UpdateTime *int64 `gorm:"column:update_time"`
}

func (UserAuth) TableName() string { return T("user_auth") }

type UserAccountLog struct {
	ID           uint    `gorm:"column:id;primaryKey" json:"id"`
	SN           string  `gorm:"column:sn" json:"sn"`
	UserID       uint    `gorm:"column:user_id" json:"user_id"`
	ChangeObject int     `gorm:"column:change_object" json:"change_object"`
	ChangeType   int     `gorm:"column:change_type" json:"change_type"`
	Action       int     `gorm:"column:action" json:"action"`
	ChangeAmount float64 `gorm:"column:change_amount" json:"change_amount"`
	LeftAmount   float64 `gorm:"column:left_amount" json:"left_amount"`
	SourceSN     string  `gorm:"column:source_sn" json:"source_sn"`
	Remark       string  `gorm:"column:remark" json:"remark"`
	Extra        string  `gorm:"column:extra" json:"extra"`
	TenantID     uint    `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime   int64   `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *int64  `gorm:"column:update_time" json:"update_time"`
	DeleteTime   *int64  `gorm:"column:delete_time" json:"delete_time"`
}

func (UserAccountLog) TableName() string { return T("user_account_log") }

type Article struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	Cid          uint   `gorm:"column:cid" json:"cid"`
	Title        string `gorm:"column:title" json:"title"`
	Desc         string `gorm:"column:desc" json:"desc"`
	Abstract     string `gorm:"column:abstract" json:"abstract"`
	Image        string `gorm:"column:image" json:"image"`
	Author       string `gorm:"column:author" json:"author"`
	Content      string `gorm:"column:content" json:"content"`
	ClickVirtual int    `gorm:"column:click_virtual" json:"click_virtual"`
	ClickActual  int    `gorm:"column:click_actual" json:"click_actual"`
	IsShow       int    `gorm:"column:is_show" json:"is_show"`
	Sort         int    `gorm:"column:sort" json:"sort"`
	TenantID     uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime   int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime   *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (Article) TableName() string { return T("article") }

type ArticleCate struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	IsShow     int    `gorm:"column:is_show" json:"is_show"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (ArticleCate) TableName() string { return T("article_cate") }

type ArticleCollect struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	UserID     uint   `gorm:"column:user_id" json:"user_id"`
	ArticleID  uint   `gorm:"column:article_id" json:"article_id"`
	Status     int    `gorm:"column:status" json:"status"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (ArticleCollect) TableName() string { return T("article_collect") }

type DecoratePage struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Type       int    `gorm:"column:type" json:"type"`
	Name       string `gorm:"column:name" json:"name"`
	Data       string `gorm:"column:data" json:"data"`
	Meta       string `gorm:"column:meta" json:"meta"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
}

func (DecoratePage) TableName() string { return T("decorate_page") }

type DecorateTabbar struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Selected   string `gorm:"column:selected" json:"selected"`
	Unselected string `gorm:"column:unselected" json:"unselected"`
	Link       string `gorm:"column:link" json:"link"`
	IsShow     int    `gorm:"column:is_show" json:"is_show"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
}

func (DecorateTabbar) TableName() string { return T("decorate_tabbar") }

type HotSearch struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Sort       int    `gorm:"column:sort" json:"sort"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

func (HotSearch) TableName() string { return T("hot_search") }

type OfficialAccountReply struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	TenantID     uint   `gorm:"column:tenant_id" json:"tenant_id"`
	Name         string `gorm:"column:name" json:"name"`
	Keyword      string `gorm:"column:keyword" json:"keyword"`
	ReplyType    int    `gorm:"column:reply_type" json:"reply_type"`
	MatchingType int    `gorm:"column:matching_type" json:"matching_type"`
	ContentType  int    `gorm:"column:content_type" json:"content_type"`
	Content      string `gorm:"column:content" json:"content"`
	Status       int    `gorm:"column:status" json:"status"`
	Sort         int    `gorm:"column:sort" json:"sort"`
	CreateTime   int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime   *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (OfficialAccountReply) TableName() string { return T("official_account_reply") }

type RechargeOrder struct {
	ID                  uint    `gorm:"column:id;primaryKey" json:"id"`
	SN                  string  `gorm:"column:sn" json:"sn"`
	UserID              uint    `gorm:"column:user_id" json:"user_id"`
	PaySN               string  `gorm:"column:pay_sn" json:"pay_sn"`
	PayWay              int     `gorm:"column:pay_way" json:"pay_way"`
	PayStatus           int     `gorm:"column:pay_status" json:"pay_status"`
	PayTime             *int64  `gorm:"column:pay_time" json:"pay_time"`
	OrderAmount         float64 `gorm:"column:order_amount" json:"order_amount"`
	OrderTerminal       int     `gorm:"column:order_terminal" json:"order_terminal"`
	TransactionID       string  `gorm:"column:transaction_id" json:"transaction_id"`
	RefundStatus        int     `gorm:"column:refund_status" json:"refund_status"`
	RefundTransactionID string  `gorm:"column:refund_transaction_id" json:"refund_transaction_id"`
	TenantID            uint    `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime          int64   `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime          *int64  `gorm:"column:update_time" json:"update_time"`
	DeleteTime          *int64  `gorm:"column:delete_time" json:"delete_time"`
}

func (RechargeOrder) TableName() string { return T("recharge_order") }

type RefundRecord struct {
	ID            uint    `gorm:"column:id;primaryKey" json:"id"`
	SN            string  `gorm:"column:sn" json:"sn"`
	UserID        uint    `gorm:"column:user_id" json:"user_id"`
	OrderID       uint    `gorm:"column:order_id" json:"order_id"`
	OrderSN       string  `gorm:"column:order_sn" json:"order_sn"`
	OrderType     string  `gorm:"column:order_type" json:"order_type"`
	OrderAmount   float64 `gorm:"column:order_amount" json:"order_amount"`
	RefundAmount  float64 `gorm:"column:refund_amount" json:"refund_amount"`
	RefundType    int     `gorm:"column:refund_type" json:"refund_type"`
	TransactionID string  `gorm:"column:transaction_id" json:"transaction_id"`
	RefundWay     int     `gorm:"column:refund_way" json:"refund_way"`
	RefundStatus  int     `gorm:"column:refund_status" json:"refund_status"`
	TenantID      uint    `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime    int64   `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime    *int64  `gorm:"column:update_time" json:"update_time"`
}

func (RefundRecord) TableName() string { return T("refund_record") }

type RefundLog struct {
	ID           uint    `gorm:"column:id;primaryKey" json:"id"`
	SN           string  `gorm:"column:sn" json:"sn"`
	RecordID     uint    `gorm:"column:record_id" json:"record_id"`
	UserID       uint    `gorm:"column:user_id" json:"user_id"`
	HandleID     uint    `gorm:"column:handle_id" json:"handle_id"`
	OrderAmount  float64 `gorm:"column:order_amount" json:"order_amount"`
	RefundAmount float64 `gorm:"column:refund_amount" json:"refund_amount"`
	RefundStatus int     `gorm:"column:refund_status" json:"refund_status"`
	RefundMsg    string  `gorm:"column:refund_msg" json:"refund_msg"`
	TenantID     uint    `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime   int64   `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *int64  `gorm:"column:update_time" json:"update_time"`
}

func (RefundLog) TableName() string { return T("refund_log") }

type PayConfig struct {
	ID     uint   `gorm:"column:id;primaryKey" json:"id"`
	Name   string `gorm:"column:name" json:"name"`
	PayWay int    `gorm:"column:pay_way" json:"pay_way"`
	Config string `gorm:"column:config" json:"config"`
	Icon   string `gorm:"column:icon" json:"icon"`
	Sort   int    `gorm:"column:sort" json:"sort"`
	Remark string `gorm:"column:remark" json:"remark"`
}

func (PayConfig) TableName() string { return T("pay_config") }

type TenantPayConfig struct {
	ID       uint   `gorm:"column:id;primaryKey" json:"id"`
	Name     string `gorm:"column:name" json:"name"`
	PayWay   int    `gorm:"column:pay_way" json:"pay_way"`
	Config   string `gorm:"column:config" json:"config"`
	Icon     string `gorm:"column:icon" json:"icon"`
	Sort     int    `gorm:"column:sort" json:"sort"`
	Remark   string `gorm:"column:remark" json:"remark"`
	TenantID uint   `gorm:"column:tenant_id" json:"tenant_id"`
}

func (TenantPayConfig) TableName() string { return T("tenant_pay_config") }

type PayWay struct {
	ID          uint `gorm:"column:id;primaryKey" json:"id"`
	PayConfigID uint `gorm:"column:pay_config_id" json:"pay_config_id"`
	Scene       int  `gorm:"column:scene" json:"scene"`
	IsDefault   int  `gorm:"column:is_default" json:"is_default"`
	Status      int  `gorm:"column:status" json:"status"`
}

func (PayWay) TableName() string { return T("pay_way") }

type TenantPayWay struct {
	ID          uint `gorm:"column:id;primaryKey" json:"id"`
	PayConfigID uint `gorm:"column:pay_config_id" json:"pay_config_id"`
	Scene       int  `gorm:"column:scene" json:"scene"`
	IsDefault   int  `gorm:"column:is_default" json:"is_default"`
	Status      int  `gorm:"column:status" json:"status"`
	TenantID    uint `gorm:"column:tenant_id" json:"tenant_id"`
}

func (TenantPayWay) TableName() string { return T("tenant_pay_way") }

type NoticeSetting struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	SceneID      int    `gorm:"column:scene_id" json:"scene_id"`
	SceneName    string `gorm:"column:scene_name" json:"scene_name"`
	SceneDesc    string `gorm:"column:scene_desc" json:"scene_desc"`
	Recipient    int    `gorm:"column:recipient" json:"recipient"`
	Type         int    `gorm:"column:type" json:"type"`
	SystemNotice string `gorm:"column:system_notice" json:"system_notice"`
	SmsNotice    string `gorm:"column:sms_notice" json:"sms_notice"`
	OaNotice     string `gorm:"column:oa_notice" json:"oa_notice"`
	MnpNotice    string `gorm:"column:mnp_notice" json:"mnp_notice"`
	Support      string `gorm:"column:support" json:"support"`
	UpdateTime   *int64 `gorm:"column:update_time" json:"update_time"`
}

func (NoticeSetting) TableName() string { return T("notice_setting") }

type TenantNoticeSetting struct {
	ID           uint   `gorm:"column:id;primaryKey" json:"id"`
	SceneID      int    `gorm:"column:scene_id" json:"scene_id"`
	SceneName    string `gorm:"column:scene_name" json:"scene_name"`
	SceneDesc    string `gorm:"column:scene_desc" json:"scene_desc"`
	Recipient    int    `gorm:"column:recipient" json:"recipient"`
	Type         int    `gorm:"column:type" json:"type"`
	SystemNotice string `gorm:"column:system_notice" json:"system_notice"`
	SmsNotice    string `gorm:"column:sms_notice" json:"sms_notice"`
	OaNotice     string `gorm:"column:oa_notice" json:"oa_notice"`
	MnpNotice    string `gorm:"column:mnp_notice" json:"mnp_notice"`
	Support      string `gorm:"column:support" json:"support"`
	TenantID     uint   `gorm:"column:tenant_id" json:"tenant_id"`
	UpdateTime   *int64 `gorm:"column:update_time" json:"update_time"`
}

func (TenantNoticeSetting) TableName() string { return T("tenant_notice_setting") }

type SmsLog struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	SceneID    int    `gorm:"column:scene_id" json:"scene_id"`
	Mobile     string `gorm:"column:mobile" json:"mobile"`
	Content    string `gorm:"column:content" json:"content"`
	Code       string `gorm:"column:code" json:"code"`
	IsVerify   int    `gorm:"column:is_verify" json:"is_verify"`
	CheckNum   int    `gorm:"column:check_num" json:"check_num"`
	SendStatus int    `gorm:"column:send_status" json:"send_status"`
	SendTime   *int64 `gorm:"column:send_time" json:"send_time"`
	Results    string `gorm:"column:results" json:"results"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (SmsLog) TableName() string { return T("sms_log") }

type TenantSmsLog struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	SceneID    int    `gorm:"column:scene_id" json:"scene_id"`
	Mobile     string `gorm:"column:mobile" json:"mobile"`
	Content    string `gorm:"column:content" json:"content"`
	Code       string `gorm:"column:code" json:"code"`
	IsVerify   int    `gorm:"column:is_verify" json:"is_verify"`
	CheckNum   int    `gorm:"column:check_num" json:"check_num"`
	SendStatus int    `gorm:"column:send_status" json:"send_status"`
	SendTime   *int64 `gorm:"column:send_time" json:"send_time"`
	Results    string `gorm:"column:results" json:"results"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantSmsLog) TableName() string { return T("tenant_sms_log") }

type NoticeRecord struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	UserID     uint   `gorm:"column:user_id" json:"user_id"`
	Title      string `gorm:"column:title" json:"title"`
	Content    string `gorm:"column:content" json:"content"`
	SceneID    int    `gorm:"column:scene_id" json:"scene_id"`
	Read       int    `gorm:"column:read" json:"read"`
	Recipient  int    `gorm:"column:recipient" json:"recipient"`
	SendType   int    `gorm:"column:send_type" json:"send_type"`
	NoticeType int    `gorm:"column:notice_type" json:"notice_type"`
	Extra      string `gorm:"column:extra" json:"extra"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (NoticeRecord) TableName() string { return T("notice_record") }

type TenantNoticeRecord struct {
	ID         uint   `gorm:"column:id;primaryKey" json:"id"`
	TenantID   uint   `gorm:"column:tenant_id" json:"tenant_id"`
	UserID     uint   `gorm:"column:user_id" json:"user_id"`
	Title      string `gorm:"column:title" json:"title"`
	Content    string `gorm:"column:content" json:"content"`
	SceneID    int    `gorm:"column:scene_id" json:"scene_id"`
	Read       int    `gorm:"column:read" json:"read"`
	Recipient  int    `gorm:"column:recipient" json:"recipient"`
	SendType   int    `gorm:"column:send_type" json:"send_type"`
	NoticeType int    `gorm:"column:notice_type" json:"notice_type"`
	Extra      string `gorm:"column:extra" json:"extra"`
	CreateTime int64  `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *int64 `gorm:"column:update_time" json:"update_time"`
	DeleteTime *int64 `gorm:"column:delete_time" json:"delete_time"`
}

func (TenantNoticeRecord) TableName() string { return T("tenant_notice_record") }
