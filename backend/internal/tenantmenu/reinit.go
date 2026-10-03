package tenantmenu

import (
	"strings"

	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
)

// Reinit copies tenant_id=0 templates onto dest the way PHP
// TenantSystemMenuLogic::initialization does (create, then remap pid).
// Child rows are inserted with the template pid; that pid is rewritten onto
// the new parent before return. EnsureTree repeats the rewrite so a missed
// update cannot leave directories such as 权限管理 with no children.
func Reinit(shared, dest *gorm.DB, tenantID uint) error {
	if dest == nil {
		dest = shared
	}
	if shared == nil {
		return nil
	}
	var tpls []model.TenantSystemMenu
	if err := shared.Where("tenant_id = 0").Order("pid, id").Find(&tpls).Error; err != nil {
		return err
	}
	if len(tpls) == 0 {
		return nil
	}
	idMap := map[uint]uint{}
	for _, m := range tpls {
		old := m.ID
		row := m
		row.ID = 0
		row.TenantID = tenantID
		now := util.NowUnix()
		row.CreateTime = now
		row.UpdateTime = util.UnixPtr(now)
		if err := dest.Session(&gorm.Session{NewDB: true}).Create(&row).Error; err != nil {
			return err
		}
		idMap[old] = row.ID
	}
	var created []model.TenantSystemMenu
	if err := dest.Session(&gorm.Session{NewDB: true}).Where("tenant_id = ?", tenantID).Find(&created).Error; err != nil {
		return err
	}
	for _, item := range created {
		if item.Pid == 0 {
			continue
		}
		nid, ok := idMap[item.Pid]
		if !ok || nid == 0 || nid == item.ID {
			continue
		}
		if err := dest.Session(&gorm.Session{NewDB: true}).Model(&model.TenantSystemMenu{}).
			Where("id = ? AND tenant_id = ?", item.ID, tenantID).
			Updates(map[string]any{"pid": nid, "update_time": util.NowUnix()}).Error; err != nil {
			return err
		}
	}
	return EnsureTree(shared, dest, tenantID)
}

// EnsureTree points child menus at this tenant's own parent rows.
// A clone that kept the template pid makes 权限管理 / 用户管理 render as
// empty directories: the sidebar never receives 菜单、角色、管理员 or 用户列表.
func EnsureTree(shared, dest *gorm.DB, tenantID uint) error {
	if tenantID == 0 || dest == nil {
		return nil
	}
	if shared == nil {
		shared = dest
	}
	var rows []model.TenantSystemMenu
	if err := dest.Session(&gorm.Session{NewDB: true}).Where("tenant_id = ?", tenantID).Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	var tpls []model.TenantSystemMenu
	if err := shared.Session(&gorm.Session{NewDB: true}).Where("tenant_id = 0").Find(&tpls).Error; err != nil {
		return err
	}
	updates := orphanPIDUpdates(tpls, rows)
	for id, pid := range updates {
		if err := dest.Session(&gorm.Session{NewDB: true}).Model(&model.TenantSystemMenu{}).
			Where("id = ? AND tenant_id = ?", id, tenantID).
			Updates(map[string]any{"pid": pid, "update_time": util.NowUnix()}).Error; err != nil {
			return err
		}
	}
	return nil
}

// orphanPIDUpdates returns row id → new pid for children whose pid still
// names a tenant_id=0 template instead of this tenant's copy of that parent.
func orphanPIDUpdates(tpls, rows []model.TenantSystemMenu) map[uint]uint {
	owned := make(map[uint]struct{}, len(rows))
	bySig := make(map[string]uint, len(rows))
	for _, row := range rows {
		owned[row.ID] = struct{}{}
		sig := menuSig(row)
		if _, ok := bySig[sig]; !ok {
			bySig[sig] = row.ID
		}
	}
	tplByID := make(map[uint]model.TenantSystemMenu, len(tpls))
	for _, tpl := range tpls {
		tplByID[tpl.ID] = tpl
	}
	updates := map[uint]uint{}
	for _, row := range rows {
		if row.Pid == 0 {
			continue
		}
		if _, ok := owned[row.Pid]; ok {
			continue
		}
		tpl, ok := tplByID[row.Pid]
		if !ok {
			continue
		}
		nid, ok := bySig[menuSig(tpl)]
		if !ok || nid == 0 || nid == row.ID {
			continue
		}
		updates[row.ID] = nid
	}
	return updates
}

func menuSig(m model.TenantSystemMenu) string {
	return strings.Join([]string{m.Type, m.Name, m.Paths, m.Perms, m.Component}, "\x00")
}
