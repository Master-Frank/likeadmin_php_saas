package middleware

import (
	"crypto/md5"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
	"time"

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
		if !renewIfNeed(c, "platform", token, info, config.C.Project.AdminToken) {
			response.AbortFail(c, "登录过期", response.CodeLoginExpire, 0)
			return
		}
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
		if !renewIfNeed(c, "tenant", token, info, config.C.Project.AdminToken) {
			response.AbortFail(c, "登录过期", response.CodeLoginExpire, 0)
			return
		}
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
		if !renewIfNeed(c, "user", token, info, config.C.Project.UserToken) {
			response.AbortFail(c, "登录过期", response.CodeLoginExpire, 0)
			return
		}
		meta.UserInfo = info
		meta.UserID = uint(util.ToInt(info["user_id"]))
	}
	c.Next()
}

func renewIfNeed(c *gin.Context, kind, token string, info map[string]any, cfg config.TokenConfig) bool {
	expire := int64(util.ToInt(info["expire_time"]))
	if expire <= 0 {
		return true
	}
	if util.NowUnix() <= expire-int64(cfg.BeExpireDuration) {
		return true
	}
	now := util.NowUnix()
	newExpire := now + int64(cfg.ExpireDuration)
	switch kind {
	case "platform":
		res := bootstrap.DB.Model(&model.AdminSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		if res.RowsAffected == 0 {
			return false
		}
		return cache.SetAdminInfo(token, ctxutil.ClientIP(c)) != nil
	case "tenant":
		res := tenantdb.Use(c).Model(&model.TenantAdminSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		if res.RowsAffected == 0 {
			return false
		}
		return cache.SetTenantAdminInfo(token, ctxutil.ClientIP(c), tenantdb.Use(c)) != nil
	case "user":
		res := tenantdb.Use(c).Model(&model.UserSession{}).Where("token = ?", token).Updates(map[string]any{"expire_time": newExpire, "update_time": now})
		if res.RowsAffected == 0 {
			return false
		}
		return cache.SetUserInfo(token, tenantdb.Use(c)) != nil
	}
	return true
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
		all = cachedURIList("admin_auth_all", func() []string {
			var menus []model.SystemMenu
			bootstrap.DB.Where("is_disable = 0 AND perms <> ''").Find(&menus)
			return collectPerms(menus)
		})
		if util.ToInt(meta.AdminInfo["root"]) == 1 {
			return all, all
		}
		urlKey := "admin_auth_url_" + strconv.FormatUint(uint64(meta.AdminID), 10)
		if cached := loadURIList(urlKey); cached != nil {
			return all, cached
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
		mine = collectPerms(allowed)
		storeURIList(urlKey, mine)
		return all, mine
	}
	db := tenantdb.Use(c)
	if db == nil {
		db = bootstrap.DB
	}
	tid := meta.TenantID
	allKey := "tenant_auth_all"
	if tid > 0 {
		allKey += "_" + strconv.FormatUint(uint64(tid), 10)
	}
	all = cachedURIList(allKey, func() []string {
		var menus []model.TenantSystemMenu
		q := db.Where("is_disable = 0 AND perms <> ''")
		if tid > 0 {
			q = q.Where("tenant_id = ?", tid)
		}
		q.Find(&menus)
		return collectTenantPerms(menus)
	})
	urlKey := "tenant_auth_url_" + strconv.FormatUint(uint64(meta.AdminID), 10)
	if tid > 0 {
		urlKey = "tenant_auth_url_" + strconv.FormatUint(uint64(tid), 10) + "_" + strconv.FormatUint(uint64(meta.AdminID), 10)
	}
	if cached := loadURIList(urlKey); cached != nil {
		return all, cached
	}
	roleIDs := toUintSlice(meta.AdminInfo["role_id"])
	if len(roleIDs) == 0 {
		return all, mine
	}
	var menuIDs []uint
	db.Model(&model.TenantSystemRoleMenu{}).Where("role_id IN ?", roleIDs).Pluck("menu_id", &menuIDs)
	var allowed []model.TenantSystemMenu
	if len(menuIDs) > 0 {
		aq := db.Where("id IN ? AND is_disable = 0 AND perms <> ''", menuIDs)
		if tid > 0 {
			aq = aq.Where("tenant_id = ?", tid)
		}
		aq.Find(&allowed)
	}
	mine = collectTenantPerms(allowed)
	storeURIList(urlKey, mine)
	return all, mine
}

func cachedURIList(key string, load func() []string) []string {
	live := load()
	md5Key := key + "_md5"
	fp := permsFingerprint(live)
	cachedFp, _ := cache.Get(md5Key)
	if cachedFp != fp {
		cache.Del(key)
		if strings.HasPrefix(key, "admin_auth_all") {
			cache.DelPrefix("admin_auth_url_")
		} else if strings.HasPrefix(key, "tenant_auth_all") {
			cache.DelPrefix("tenant_auth_url_")
		}
		cache.Set(md5Key, fp, time.Hour)
		storeURIList(key, live)
		return live
	}
	if cached := loadURIList(key); cached != nil {
		return cached
	}
	storeURIList(key, live)
	return live
}

func permsFingerprint(uris []string) string {
	cp := append([]string(nil), uris...)
	sort.Strings(cp)
	sum := md5.Sum([]byte(strings.Join(cp, "\n")))
	return hex.EncodeToString(sum[:])
}

func loadURIList(key string) []string {
	var out []string
	if cache.GetJSON(key, &out) && out != nil {
		return out
	}
	return nil
}

func storeURIList(key string, uris []string) {
	if len(uris) == 0 {
		return
	}
	cache.Set(key, uris, time.Hour)
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
