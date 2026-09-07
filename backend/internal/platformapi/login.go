package platformapi

import (
	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const platformLockTag = `app\common\cache\AdminAccountSafeCache`

func LoginAccount(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if msg := util.LoginTerminalCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	account := httpx.BodyStr(c, "account")
	password := httpx.BodyStr(c, "password")
	terminal := httpx.BodyInt(c, "terminal")
	if account == "" {
		response.Fail(c, "请输入账号")
		return
	}
	if password == "" {
		response.Fail(c, "请输入密码")
		return
	}
	restrict := cfgsvc.GetInt(c, "admin_login", "login_restrictions", 1)
	times := cfgsvc.GetInt(c, "admin_login", "password_error_times", 5)
	limit := cfgsvc.GetInt(c, "admin_login", "limit_login_time", 30)
	if restrict == 1 {
		if ok, msg := authsvc.CheckLoginLock(c, platformLockTag, times, limit); !ok {
			response.Fail(c, msg)
			return
		}
	}
	var admin model.Admin
	err := bootstrap.DB.Where("account = ? AND delete_time IS NULL", account).First(&admin).Error
	if err != nil {
		response.Fail(c, "账号不存在")
		return
	}
	if admin.Disable == 1 {
		response.Fail(c, "账号已禁用")
		return
	}
	if admin.Password == "" {
		if restrict == 1 {
			authsvc.RecordLoginFail(c, platformLockTag, limit)
		}
		response.Fail(c, "账号不存在")
		return
	}
	if admin.Password != util.CreatePassword(password, config.C.Project.UniqueIdentification) {
		if restrict == 1 {
			authsvc.RecordLoginFail(c, platformLockTag, limit)
		}
		response.Fail(c, "密码错误")
		return
	}
	if restrict == 1 {
		authsvc.RelieveLoginFail(c, platformLockTag)
	}
	now := util.NowUnix()
	ip := ctxutil.ClientIP(c)
	bootstrap.DB.Model(&admin).Updates(map[string]any{"login_time": now, "login_ip": ip, "update_time": now})
	info := authsvc.SetPlatformToken(c, admin.ID, terminal, admin.MultipointLogin)
	avatar := admin.Avatar
	if avatar == "" {
		avatar = config.C.Project.DefaultImage["admin_avatar"]
	}
	response.Data(c, gin.H{
		"name":      info["name"],
		"avatar":    filesvc.GetFileURL(c, avatar),
		"role_name": info["role_name"],
		"token":     info["token"],
	})
}

func LoginLogout(c *gin.Context) {
	meta := ctxutil.Get(c)
	if meta.AdminInfo != nil {
		if token := util.ToString(meta.AdminInfo["token"]); token != "" {
			authsvc.ExpirePlatformToken(token)
		}
	}
	response.Success(c, "success", nil)
}
