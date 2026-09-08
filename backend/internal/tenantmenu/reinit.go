package tenantmenu

import (
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
)

// Reinit copies tenant_id=0 templates onto dest the way PHP
// TenantSystemMenuLogic::initialization does (create, then remap pid).
func Reinit(shared, dest *gorm.DB, tenantID uint) error {
	if dest == nil {
		dest = shared
	}
	if shared == nil {
		return nil
	}
	var tpls []model.TenantSystemMenu
	shared.Where("tenant_id = 0").Order("pid, id").Find(&tpls)
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
		if err := dest.Create(&row).Error; err != nil {
			return err
		}
		idMap[old] = row.ID
	}
	var created []model.TenantSystemMenu
	dest.Where("tenant_id = ?", tenantID).Find(&created)
	for _, item := range created {
		if item.Pid != 0 {
			if nid, ok := idMap[item.Pid]; ok {
				dest.Model(&item).Updates(map[string]any{"pid": nid, "update_time": util.NowUnix()})
			}
		}
	}
	return nil
}
