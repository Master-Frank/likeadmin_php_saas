package tenantmenu

import (
	"testing"

	"likeadmin/backend/internal/model"
)

func TestReinitNilShared(t *testing.T) {
	if err := Reinit(nil, nil, 1); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureTreeNil(t *testing.T) {
	if err := EnsureTree(nil, nil, 1); err != nil {
		t.Fatal(err)
	}
	if err := EnsureTree(nil, nil, 0); err != nil {
		t.Fatal(err)
	}
}

func TestOrphanPIDUpdatesReparentsTemplateChildren(t *testing.T) {
	tpls := []model.TenantSystemMenu{
		{ID: 4, Pid: 0, Type: "M", Name: "权限管理", Paths: "permission"},
		{ID: 6, Pid: 4, Type: "C", Name: "菜单", Paths: "menu", Perms: "auth.menu/lists", Component: "permission/menu/index"},
		{ID: 8, Pid: 4, Type: "C", Name: "角色", Paths: "role", Perms: "auth.role/lists", Component: "permission/role/index"},
		{ID: 7, Pid: 4, Type: "C", Name: "管理员", Paths: "admin", Perms: "auth.admin/lists", Component: "permission/admin/index"},
		{ID: 117, Pid: 0, Type: "M", Name: "用户管理", Paths: "consumer"},
		{ID: 118, Pid: 117, Type: "C", Name: "用户列表", Paths: "lists", Perms: "user.user/lists", Component: "consumer/lists/index"},
		{ID: 28, Pid: 0, Type: "M", Name: "系统设置", Paths: "setting"},
		{ID: 29, Pid: 28, Type: "M", Name: "网站设置", Paths: "website"},
		{ID: 30, Pid: 29, Type: "C", Name: "网站信息", Paths: "information", Component: "setting/website/information"},
	}
	rows := []model.TenantSystemMenu{
		{ID: 200, Pid: 0, TenantID: 1, Type: "M", Name: "权限管理", Paths: "permission"},
		{ID: 201, Pid: 4, TenantID: 1, Type: "C", Name: "菜单", Paths: "menu", Perms: "auth.menu/lists", Component: "permission/menu/index"},
		{ID: 202, Pid: 4, TenantID: 1, Type: "C", Name: "角色", Paths: "role", Perms: "auth.role/lists", Component: "permission/role/index"},
		{ID: 203, Pid: 4, TenantID: 1, Type: "C", Name: "管理员", Paths: "admin", Perms: "auth.admin/lists", Component: "permission/admin/index"},
		{ID: 210, Pid: 0, TenantID: 1, Type: "M", Name: "用户管理", Paths: "consumer"},
		{ID: 211, Pid: 117, TenantID: 1, Type: "C", Name: "用户列表", Paths: "lists", Perms: "user.user/lists", Component: "consumer/lists/index"},
		{ID: 300, Pid: 0, TenantID: 1, Type: "M", Name: "系统设置", Paths: "setting"},
		{ID: 301, Pid: 28, TenantID: 1, Type: "M", Name: "网站设置", Paths: "website"},
		{ID: 302, Pid: 29, TenantID: 1, Type: "C", Name: "网站信息", Paths: "information", Component: "setting/website/information"},
		{ID: 5, Pid: 0, TenantID: 1, Type: "C", Name: "工作台", Paths: "workbench", Component: "workbench/index"},
	}
	got := orphanPIDUpdates(tpls, rows)
	want := map[uint]uint{
		201: 200, 202: 200, 203: 200,
		211: 210,
		301: 300, 302: 301,
	}
	if len(got) != len(want) {
		t.Fatalf("updates %v want %v", got, want)
	}
	for id, pid := range want {
		if got[id] != pid {
			t.Fatalf("id %d pid %d want %d", id, got[id], pid)
		}
	}
}

func TestOrphanPIDUpdatesLeavesResolvedTree(t *testing.T) {
	tpls := []model.TenantSystemMenu{
		{ID: 4, Pid: 0, Type: "M", Name: "权限管理", Paths: "permission"},
		{ID: 6, Pid: 4, Type: "C", Name: "菜单", Paths: "menu", Perms: "auth.menu/lists", Component: "permission/menu/index"},
	}
	// Sharded clones keep the template ids, so pid already names a row in this tenant.
	rows := []model.TenantSystemMenu{
		{ID: 4, Pid: 0, TenantID: 2, Type: "M", Name: "权限管理", Paths: "permission"},
		{ID: 6, Pid: 4, TenantID: 2, Type: "C", Name: "菜单", Paths: "menu", Perms: "auth.menu/lists", Component: "permission/menu/index"},
	}
	if got := orphanPIDUpdates(tpls, rows); len(got) != 0 {
		t.Fatalf("resolved tree must not be rewritten: %v", got)
	}
	rows[1].Pid = 4
	rows[0].ID = 200
	rows[1].ID = 201
	rows[1].Pid = 200
	if got := orphanPIDUpdates(tpls, rows); len(got) != 0 {
		t.Fatalf("already remapped pid must stay: %v", got)
	}
}
