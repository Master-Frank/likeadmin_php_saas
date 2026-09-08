package export

import "strings"

type Field struct {
	Key   string
	Title string
}

type Spec struct {
	FileName string
	Fields   []Field
}

var excelSpecs = map[string]Spec{
	"setting.system.log/lists": {
		FileName: "系统日志",
		Fields: []Field{
			{Key: "id", Title: "记录ID"},
			{Key: "action", Title: "操作"},
			{Key: "admin_name", Title: "管理员"},
			{Key: "admin_id", Title: "管理员ID"},
			{Key: "url", Title: "访问链接"},
			{Key: "type", Title: "访问方式"},
			{Key: "params", Title: "访问参数"},
			{Key: "ip", Title: "来源IP"},
			{Key: "create_time", Title: "日志时间"},
		},
	},
	"tenant.tenant/lists": {
		FileName: "租户列表",
		Fields: []Field{
			{Key: "sn", Title: "租户编号"},
			{Key: "name", Title: "租户昵称"},
			{Key: "disable", Title: "租户状态"},
			{Key: "create_time", Title: "注册时间"},
		},
	},
	"tenant.tenant_admin/lists": {
		FileName: "租户用户列表",
		Fields: []Field{
			{Key: "root", Title: "是否超级管理员"},
			{Key: "name", Title: "租户账户昵称"},
			{Key: "avatar", Title: "头像"},
			{Key: "account", Title: "账号"},
			{Key: "multipoint_login", Title: "是否允许多处登录"},
			{Key: "disable", Title: "是否禁用"},
			{Key: "create_time", Title: "注册时间"},
		},
	},
	"auth.admin/lists": {
		FileName: "管理员列表",
		Fields: []Field{
			{Key: "account", Title: "账号"},
			{Key: "name", Title: "名称"},
			{Key: "role_name", Title: "角色"},
			{Key: "dept_name", Title: "部门"},
			{Key: "create_time", Title: "创建时间"},
			{Key: "login_time", Title: "最近登录时间"},
			{Key: "login_ip", Title: "最近登录IP"},
			{Key: "disable_desc", Title: "状态"},
		},
	},
	"dept.jobs/lists": {
		FileName: "岗位列表",
		Fields: []Field{
			{Key: "code", Title: "岗位编码"},
			{Key: "name", Title: "岗位名称"},
			{Key: "remark", Title: "备注"},
			{Key: "status_desc", Title: "状态"},
			{Key: "create_time", Title: "添加时间"},
		},
	},
	"user.user/lists": {
		FileName: "用户列表",
		Fields: []Field{
			{Key: "sn", Title: "用户编号"},
			{Key: "nickname", Title: "用户昵称"},
			{Key: "account", Title: "账号"},
			{Key: "mobile", Title: "手机号码"},
			{Key: "channel", Title: "注册来源"},
			{Key: "create_time", Title: "注册时间"},
		},
	},
	"recharge.recharge/lists": {
		FileName: "充值记录",
		Fields: []Field{
			{Key: "sn", Title: "充值单号"},
			{Key: "nickname", Title: "用户昵称"},
			{Key: "order_amount", Title: "充值金额"},
			{Key: "pay_way_text", Title: "支付方式"},
			{Key: "pay_status_text", Title: "支付状态"},
			{Key: "pay_time", Title: "支付时间"},
			{Key: "create_time", Title: "下单时间"},
		},
	},
}

func Lookup(controller, action string) Spec {
	key := strings.ToLower(strings.TrimSpace(controller) + "/" + strings.TrimSpace(action))
	if spec, ok := excelSpecs[key]; ok {
		return spec
	}
	compact := strings.ReplaceAll(key, "_", "")
	for k, spec := range excelSpecs {
		if strings.ReplaceAll(k, "_", "") == compact {
			return spec
		}
	}
	return Spec{}
}
