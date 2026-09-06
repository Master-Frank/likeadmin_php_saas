package middleware

import (
	"strings"

	"likeadmin/backend/internal/authsvc"
	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"
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
		all, mine := adminURIs(c, meta)
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
		response.AbortFail(c, "演示环境不支持修改数据，请下载源码本地部署体验", response.CodeFail, 1)
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
		response.AbortFail(c, "登录超时，请重新登录", response.CodeLoginExpire, 0)
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
		info = cache.GetTenantAdminInfo(token, ctxutil.ClientIP(c), tenantdb.Use(c))
	}
	if (info == nil || len(info) == 0) && need {
		response.AbortFail(c, "登录超时，请重新登录", response.CodeLoginExpire, 0)
		return
	}
	if info != nil && len(info) > 0 {
		if rejectWrongTenant(need, uint(util.ToInt(info["tenant_id"])), meta.TenantID) {
			authsvc.ExpireTenantToken(c, token)
			response.AbortFail(c, "非该站点成员禁止访问", response.CodeLoginExpire, 1)
			return
		}
		renewIfNeed(c, "tenant", token, info, config.C.Project.AdminToken)
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
		info = cache.GetUserInfo(token, tenantdb.Use(c))
	}
	if (info == nil || len(info) == 0) && need {
		response.AbortFail(c, "登录超时，请重新登录", response.CodeLoginExpire, 0)
		return
	}
	if info != nil && len(info) > 0 {
		if rejectWrongTenant(need, uint(util.ToInt(info["tenant_id"])), meta.TenantID) {
			authsvc.ExpireUserToken(c, token)
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
		tenantdb.Use(c).Model(&model.TenantAdminSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		cache.SetTenantAdminInfo(token, ctxutil.ClientIP(c), tenantdb.Use(c))
	case "user":
		tenantdb.Use(c).Model(&model.UserSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		cache.SetUserInfo(token, tenantdb.Use(c))
	}
}

// rejectWrongTenant matches PHP LoginMiddleware: only required-login
// routes abort on a stale cross-tenant token. Optional routes still run.
func rejectWrongTenant(need bool, tokenTenant, hostTenant uint) bool {
	if !need || hostTenant == 0 || tokenTenant == hostTenant {
		return false
	}
	return true
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

func adminURIs(c *gin.Context, meta *ctxutil.RequestMeta) (all, mine []string) {
	// PHP AuthLogic uses menu.perms (API URIs), not frontend paths.
	if meta.App == "platformapi" {
		var menus []model.SystemMenu
		bootstrap.DB.Where("is_disable = 0 AND perms <> ''").Find(&menus)
		all = collectPerms(menus)
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
			bootstrap.DB.Where("id IN ? AND is_disable = 0 AND perms <> ''", menuIDs).Find(&allowed)
		}
		return all, collectPerms(allowed)
	}
	db := tenantdb.Use(c)
	if db == nil {
		db = bootstrap.DB
	}
	var menus []model.TenantSystemMenu
	q := db.Where("is_disable = 0 AND perms <> ''")
	if meta.TenantID > 0 {
		q = q.Where("tenant_id = ?", meta.TenantID)
	}
	q.Find(&menus)
	all = collectTenantPerms(menus)
	roleIDs := toUintSlice(meta.AdminInfo["role_id"])
	if len(roleIDs) == 0 {
		return all, mine
	}
	var menuIDs []uint
	db.Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
	var allowed []model.TenantSystemMenu
	if len(menuIDs) > 0 {
		aq := db.Where("id IN ? AND is_disable = 0 AND perms <> ''", menuIDs)
		if meta.TenantID > 0 {
			aq = aq.Where("tenant_id = ?", meta.TenantID)
		}
		aq.Find(&allowed)
	}
	return all, collectTenantPerms(allowed)
}

func collectPerms(menus []model.SystemMenu) []string {
	out := make([]string, 0, len(menus))
	for _, m := range menus {
		if m.Perms != "" {
			out = append(out, formatURI(m.Perms))
		}
	}
	return out
}

func collectTenantPerms(menus []model.TenantSystemMenu) []string {
	out := make([]string, 0, len(menus))
	for _, m := range menus {
		if m.Perms != "" {
			out = append(out, formatURI(m.Perms))
		}
	}
	return out
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
