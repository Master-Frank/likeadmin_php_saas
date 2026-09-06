package middleware

import (
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func Login(notNeed map[string][]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		meta := ctxutil.Get(c)
		token := c.GetHeader("token")
		if token == "null" {
			token = ""
		}
		need := !isNotNeed(notNeed, meta)
		switch meta.App {
		case "platformapi":
			handlePlatformLogin(c, meta, token, need)
		case "tenantapi":
			handleTenantLogin(c, meta, token, need)
		case "api":
			handleUserLogin(c, meta, token, need)
		default:
			c.Next()
		}
	}
}

func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		meta := ctxutil.Get(c)
		if meta.NotNeedLogin {
			c.Next()
			return
		}
		if meta.App != "platformapi" && meta.App != "tenantapi" {
			c.Next()
			return
		}
		if meta.AdminInfo == nil {
			c.Next()
			return
		}
		if util.ToInt(meta.AdminInfo["root"]) == 1 {
			c.Next()
			return
		}
		loginIP := util.ToString(meta.AdminInfo["login_ip"])
		if loginIP != "" && loginIP != ctxutil.ClientIP(c) {
			response.AbortFail(c, "ip地址发生变化，请重新登录", response.CodeLoginExpire, 1)
			return
		}
		accessURI := strings.ToLower(meta.Controller + "/" + meta.Action)
		all, mine := adminURIs(meta)
		if !containsURI(all, accessURI) {
			c.Next()
			return
		}
		if containsURI(mine, accessURI) {
			c.Next()
			return
		}
		response.AbortFail(c, "权限不足，无法访问或操作", response.CodeFail, 1)
	}
}

func DemoGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.C.Project.DemoEnv {
			c.Next()
			return
		}
		if c.Request.Method == "GET" {
			c.Next()
			return
		}
		meta := ctxutil.Get(c)
		if meta.NotNeedLogin && (meta.Action == "account" || meta.Action == "logout") {
			c.Next()
			return
		}
		response.AbortFail(c, "演示环境请勿修改", response.CodeFail, 1)
	}
}

func handlePlatformLogin(c *gin.Context, meta *ctxutil.RequestMeta, token string, need bool) {
	if token == "" && need {
		response.AbortFail(c, "请求参数缺token", response.CodeFail, 0)
		return
	}
	var info map[string]any
	if token != "" {
		info = cache.GetAdminInfo(token, ctxutil.ClientIP(c))
	}
	if (info == nil || len(info) == 0) && need {
		response.AbortFail(c, "登录超时，请重新登录", response.CodeLoginExpire, 1)
		return
	}
	if info != nil && len(info) > 0 {
		renewIfNeed(c, "platform", token, info, config.C.Project.AdminToken)
		meta.AdminInfo = info
		meta.AdminID = uint(util.ToInt(info["admin_id"]))
	}
	c.Next()
}

func handleTenantLogin(c *gin.Context, meta *ctxutil.RequestMeta, token string, need bool) {
	if token == "" && need {
		response.AbortFail(c, "请求参数缺token", response.CodeFail, 0)
		return
	}
	var info map[string]any
	if token != "" {
		info = cache.GetTenantAdminInfo(token, ctxutil.ClientIP(c))
	}
	if (info == nil || len(info) == 0) && need {
		response.AbortFail(c, "登录超时，请重新登录", response.CodeLoginExpire, 1)
		return
	}
	if info != nil && len(info) > 0 {
		if meta.TenantID > 0 && uint(util.ToInt(info["tenant_id"])) != meta.TenantID {
			response.AbortFail(c, "非该站点成员禁止访问", response.CodeLoginExpire, 1)
			return
		}
		renewIfNeed(c, "tenant", token, info, config.C.Project.TenantToken)
		meta.AdminInfo = info
		meta.AdminID = uint(util.ToInt(info["admin_id"]))
	}
	c.Next()
}

func handleUserLogin(c *gin.Context, meta *ctxutil.RequestMeta, token string, need bool) {
	if token == "" && need {
		response.AbortFail(c, "请求参数缺token", response.CodeFail, 0)
		return
	}
	var info map[string]any
	if token != "" {
		info = cache.GetUserInfo(token)
	}
	if (info == nil || len(info) == 0) && need {
		response.AbortFail(c, "登录超时，请重新登录", response.CodeLoginExpire, 1)
		return
	}
	if info != nil && len(info) > 0 {
		if meta.TenantID > 0 && uint(util.ToInt(info["tenant_id"])) != meta.TenantID {
			response.AbortFail(c, "非该站点用户禁止访问", response.CodeLoginExpire, 1)
			return
		}
		renewIfNeed(c, "user", token, info, config.C.Project.UserToken)
		meta.UserInfo = info
		meta.UserID = uint(util.ToInt(info["user_id"]))
	}
	c.Next()
}

func renewIfNeed(c *gin.Context, kind, token string, info map[string]any, cfg config.TokenConfig) {
	expire := int64(util.ToInt(info["expire_time"]))
	if expire <= 0 {
		return
	}
	if util.NowUnix() <= expire-int64(cfg.BeExpireDuration) {
		return
	}
	now := util.NowUnix()
	newExpire := now + int64(cfg.ExpireDuration)
	switch kind {
	case "platform":
		bootstrap.DB.Model(&model.AdminSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		cache.SetAdminInfo(token, ctxutil.ClientIP(c))
	case "tenant":
		bootstrap.DB.Model(&model.TenantAdminSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		cache.SetTenantAdminInfo(token, ctxutil.ClientIP(c))
	case "user":
		bootstrap.DB.Model(&model.UserSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		cache.SetUserInfo(token)
	}
}

func isNotNeed(table map[string][]string, meta *ctxutil.RequestMeta) bool {
	key := strings.ToLower(meta.Controller)
	actions := table[key]
	if util.InFold(actions, meta.Action) {
		meta.NotNeedLogin = true
		return true
	}
	// also try without case
	for k, acts := range table {
		if strings.EqualFold(k, meta.Controller) && util.InFold(acts, meta.Action) {
			meta.NotNeedLogin = true
			return true
		}
	}
	return false
}

func adminURIs(meta *ctxutil.RequestMeta) (all, mine []string) {
	adminID := meta.AdminID
	if meta.App == "platformapi" {
		var menus []model.SystemMenu
		bootstrap.DB.Where("paths <> ''").Find(&menus)
		for _, m := range menus {
			if m.Paths != "" {
				all = append(all, formatURI(m.Paths))
			}
		}
		if util.ToInt(meta.AdminInfo["root"]) == 1 {
			return all, all
		}
		roleIDs := toUintSlice(meta.AdminInfo["role_id"])
		if len(roleIDs) == 0 {
			return all, mine
		}
		var menuIDs []uint
		bootstrap.DB.Model(&model.SystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
		var allowed []model.SystemMenu
		if len(menuIDs) > 0 {
			bootstrap.DB.Where("id IN ?", menuIDs).Find(&allowed)
		}
		for _, m := range allowed {
			if m.Paths != "" {
				mine = append(mine, formatURI(m.Paths))
			}
		}
		_ = adminID
		return all, mine
	}
	var menus []model.TenantSystemMenu
	q := bootstrap.DB.Where("paths <> ''")
	if meta.TenantID > 0 {
		q = q.Where("tenant_id = ?", meta.TenantID)
	}
	q.Find(&menus)
	for _, m := range menus {
		if m.Paths != "" {
			all = append(all, formatURI(m.Paths))
		}
	}
	roleIDs := toUintSlice(meta.AdminInfo["role_id"])
	if len(roleIDs) == 0 {
		return all, mine
	}
	var menuIDs []uint
	bootstrap.DB.Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
	var allowed []model.TenantSystemMenu
	if len(menuIDs) > 0 {
		bootstrap.DB.Where("id IN ?", menuIDs).Find(&allowed)
	}
	for _, m := range allowed {
		if m.Paths != "" {
			mine = append(mine, formatURI(m.Paths))
		}
	}
	return all, mine
}

func formatURI(path string) string {
	return strings.ToLower(util.ToCamelLower(path))
}

func containsURI(list []string, uri string) bool {
	uri = strings.ToLower(uri)
	uri2 := strings.ReplaceAll(uri, "_", "")
	for _, item := range list {
		if item == uri || strings.ReplaceAll(item, "_", "") == uri2 {
			return true
		}
	}
	return false
}

func toUintSlice(v any) []uint {
	switch t := v.(type) {
	case []int:
		out := make([]uint, len(t))
		for i, n := range t {
			out[i] = uint(n)
		}
		return out
	case []uint:
		return t
	case []any:
		out := make([]uint, 0, len(t))
		for _, item := range t {
			out = append(out, uint(util.ToInt(item)))
		}
		return out
	default:
		return nil
	}
}
