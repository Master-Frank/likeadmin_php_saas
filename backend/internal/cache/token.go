package cache

import (
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
)

func useDB(db *gorm.DB) *gorm.DB {
	if db != nil {
		return db
	}
	return bootstrap.DB
}

func GetAdminInfo(token string, ip string) map[string]any {
	key := "token_admin_" + token
	var cached map[string]any
	if GetJSON(key, &cached) && len(cached) > 0 {
		return cached
	}
	return SetAdminInfo(token, ip)
}

func SetAdminInfo(token, ip string) map[string]any {
	var sess model.AdminSession
	now := util.NowUnix()
	if err := bootstrap.DB.Where("token = ? AND expire_time > ?", token, now).First(&sess).Error; err != nil {
		return nil
	}
	var admin model.Admin
	if err := bootstrap.DB.Where("id = ? AND delete_time IS NULL", sess.AdminID).First(&admin).Error; err != nil {
		return nil
	}
	roleIDs := []int{}
	bootstrap.DB.Model(&model.AdminRole{}).Where("admin_id = ?", admin.ID).Pluck("role_id", &roleIDs)
	roleName := ""
	if admin.Root == 1 {
		roleName = "系统管理员"
	} else {
		var roles []model.SystemRole
		if len(roleIDs) > 0 {
			bootstrap.DB.Where("id IN ? AND delete_time IS NULL", roleIDs).Find(&roles)
		}
		for i, r := range roles {
			if i > 0 {
				roleName += "/"
			}
			roleName += r.Name
		}
	}
	info := map[string]any{
		"admin_id":    admin.ID,
		"root":        admin.Root,
		"name":        admin.Name,
		"account":     admin.Account,
		"role_name":   roleName,
		"role_id":     roleIDs,
		"token":       token,
		"terminal":    sess.Terminal,
		"expire_time": sess.ExpireTime,
		"login_ip":    ip,
	}
	ttl := time.Until(time.Unix(sess.ExpireTime, 0))
	if ttl > 0 {
		Set("token_admin_"+token, info, ttl)
	}
	return info
}

func DeleteAdminInfo(token string) {
	Del("token_admin_" + token)
}

func GetTenantAdminInfo(token, ip string, db *gorm.DB) map[string]any {
	key := "token_tenant_" + token
	var cached map[string]any
	if GetJSON(key, &cached) && len(cached) > 0 {
		return cached
	}
	return SetTenantAdminInfo(token, ip, db)
}

func SetTenantAdminInfo(token, ip string, db *gorm.DB) map[string]any {
	db = useDB(db)
	var sess model.TenantAdminSession
	now := util.NowUnix()
	if err := db.Where("token = ? AND expire_time > ?", token, now).First(&sess).Error; err != nil {
		return nil
	}
	var admin model.TenantAdmin
	if err := db.Where("id = ? AND delete_time IS NULL", sess.AdminID).First(&admin).Error; err != nil {
		return nil
	}
	roleIDs := []int{}
	db.Model(&model.TenantAdminRole{}).Where("admin_id = ?", admin.ID).Pluck("role_id", &roleIDs)
	roleName := ""
	if admin.Root == 1 {
		roleName = "系统管理员"
	} else {
		var roles []model.TenantSystemRole
		if len(roleIDs) > 0 {
			db.Where("id IN ? AND delete_time IS NULL", roleIDs).Find(&roles)
		}
		for i, r := range roles {
			if i > 0 {
				roleName += "/"
			}
			roleName += r.Name
		}
	}
	info := map[string]any{
		"admin_id":    admin.ID,
		"tenant_id":   admin.TenantID,
		"root":        admin.Root,
		"name":        admin.Name,
		"account":     admin.Account,
		"role_name":   roleName,
		"role_id":     roleIDs,
		"token":       token,
		"terminal":    sess.Terminal,
		"expire_time": sess.ExpireTime,
		"login_ip":    ip,
	}
	ttl := time.Until(time.Unix(sess.ExpireTime, 0))
	if ttl > 0 {
		Set("token_tenant_"+token, info, ttl)
	}
	return info
}

func DeleteTenantAdminInfo(token string) {
	Del("token_tenant_" + token)
}

func GetUserInfo(token string, db *gorm.DB) map[string]any {
	key := "token_user_" + token
	var cached map[string]any
	if GetJSON(key, &cached) && len(cached) > 0 {
		return cached
	}
	return SetUserInfo(token, db)
}

func SetUserInfo(token string, db *gorm.DB) map[string]any {
	db = useDB(db)
	var sess model.UserSession
	now := util.NowUnix()
	if err := db.Where("token = ? AND expire_time > ?", token, now).First(&sess).Error; err != nil {
		if err != gorm.ErrRecordNotFound {
			return nil
		}
		return nil
	}
	var user model.User
	if err := db.Where("id = ? AND delete_time IS NULL", sess.UserID).First(&user).Error; err != nil {
		return nil
	}
	info := map[string]any{
		"user_id":     user.ID,
		"tenant_id":   user.TenantID,
		"nickname":    user.Nickname,
		"sn":          user.SN,
		"mobile":      user.Mobile,
		"avatar":      user.Avatar,
		"token":       token,
		"terminal":    sess.Terminal,
		"expire_time": sess.ExpireTime,
	}
	ttl := time.Until(time.Unix(sess.ExpireTime, 0))
	if ttl > 0 {
		Set("token_user_"+token, info, ttl)
	}
	return info
}

func DeleteUserInfo(token string) {
	Del("token_user_" + token)
}

func LoginLockKey(tag, ip string) string {
	return tag + ip
}
