package authsvc

import (
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetPlatformToken(c *gin.Context, adminID uint, terminal, multipoint int) map[string]any {
	now := util.NowUnix()
	expire := now + int64(config.C.Project.AdminToken.ExpireDuration)
	var sess model.AdminSession
	err := bootstrap.DB.Where("admin_id = ? AND terminal = ?", adminID, terminal).First(&sess).Error
	token := util.CreateToken(util.ToString(adminID), config.C.Project.UniqueIdentification)
	if err == nil {
		if sess.ExpireTime < now || multipoint == 0 {
			cache.DeleteAdminInfo(sess.Token)
			sess.Token = token
		}
		sess.ExpireTime = expire
		sess.UpdateTime = &now
		bootstrap.DB.Save(&sess)
	} else if err == gorm.ErrRecordNotFound {
		sess = model.AdminSession{AdminID: adminID, Terminal: terminal, Token: token, ExpireTime: expire, UpdateTime: &now}
		bootstrap.DB.Create(&sess)
	}
	return cache.SetAdminInfo(sess.Token, ctxutil.ClientIP(c))
}

func ExpirePlatformToken(token string) bool {
	var sess model.AdminSession
	if err := bootstrap.DB.Where("token = ?", token).First(&sess).Error; err != nil {
		return false
	}
	var admin model.Admin
	if err := bootstrap.DB.Where("id = ?", sess.AdminID).First(&admin).Error; err != nil {
		return false
	}
	if admin.MultipointLogin == 1 {
		return false
	}
	now := util.NowUnix()
	sess.ExpireTime = now
	sess.UpdateTime = &now
	bootstrap.DB.Save(&sess)
	cache.DeleteAdminInfo(token)
	return true
}

func SetTenantToken(c *gin.Context, adminID uint, terminal, multipoint int) map[string]any {
	now := util.NowUnix()
	expire := now + int64(config.C.Project.TenantToken.ExpireDuration)
	var sess model.TenantAdminSession
	err := bootstrap.DB.Where("admin_id = ? AND terminal = ?", adminID, terminal).First(&sess).Error
	token := util.CreateToken(util.ToString(adminID), config.C.Project.UniqueIdentification)
	if err == nil {
		if sess.ExpireTime < now || multipoint == 0 {
			cache.DeleteTenantAdminInfo(sess.Token)
			sess.Token = token
		}
		sess.ExpireTime = expire
		sess.UpdateTime = &now
		bootstrap.DB.Save(&sess)
	} else if err == gorm.ErrRecordNotFound {
		sess = model.TenantAdminSession{AdminID: adminID, Terminal: terminal, Token: token, ExpireTime: expire, UpdateTime: &now}
		bootstrap.DB.Create(&sess)
	}
	return cache.SetTenantAdminInfo(sess.Token, ctxutil.ClientIP(c))
}

func ExpireTenantToken(token string) bool {
	var sess model.TenantAdminSession
	if err := bootstrap.DB.Where("token = ?", token).First(&sess).Error; err != nil {
		return false
	}
	var admin model.TenantAdmin
	if err := bootstrap.DB.Where("id = ?", sess.AdminID).First(&admin).Error; err != nil {
		return false
	}
	if admin.MultipointLogin == 1 {
		return false
	}
	now := util.NowUnix()
	sess.ExpireTime = now
	sess.UpdateTime = &now
	bootstrap.DB.Save(&sess)
	cache.DeleteTenantAdminInfo(token)
	return true
}

func SetUserToken(c *gin.Context, userID uint, terminal int) map[string]any {
	now := util.NowUnix()
	expire := now + int64(config.C.Project.UserToken.ExpireDuration)
	var sess model.UserSession
	err := bootstrap.DB.Where("user_id = ? AND terminal = ?", userID, terminal).First(&sess).Error
	token := util.CreateToken(util.ToString(userID), config.C.Project.UniqueIdentification)
	if err == nil {
		if sess.ExpireTime < now {
			cache.DeleteUserInfo(sess.Token)
			sess.Token = token
		}
		sess.ExpireTime = expire
		sess.UpdateTime = &now
		bootstrap.DB.Save(&sess)
	} else {
		sess = model.UserSession{UserID: userID, Terminal: terminal, Token: token, ExpireTime: expire, UpdateTime: &now}
		bootstrap.DB.Create(&sess)
	}
	return cache.SetUserInfo(sess.Token)
}

func ExpireUserToken(token string) {
	now := util.NowUnix()
	bootstrap.DB.Model(&model.UserSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": now, "update_time": now})
	cache.DeleteUserInfo(token)
}

func CheckLoginLock(c *gin.Context, tag string, count, minute int) (bool, string) {
	if count <= 0 {
		count = 5
	}
	if minute <= 0 {
		minute = 30
	}
	key := tag + ctxutil.ClientIP(c)
	n := 0
	if raw, ok := cache.Get(key); ok {
		n = util.ParseInt(raw)
	}
	if n >= count {
		return false, "密码连续" + util.ToString(count) + "次输入错误，请" + util.ToString(minute) + "分钟后重试"
	}
	return true, ""
}

func RecordLoginFail(c *gin.Context, tag string, minute int) {
	if minute <= 0 {
		minute = 30
	}
	key := tag + ctxutil.ClientIP(c)
	n := cache.Incr(key)
	if n == 1 {
		cache.Expire(key, time.Duration(minute)*time.Minute)
	}
}

func RelieveLoginFail(c *gin.Context, tag string) {
	cache.Del(tag + ctxutil.ClientIP(c))
}
