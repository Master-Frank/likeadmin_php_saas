package middleware

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/tenantdb"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Sec-Fetch-Mode, DNT, X-Mx-ReqToken, Keep-Alive, User-Agent, If-Match, If-None-Match, If-Unmodified-Since, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Accept-Language, Origin, Accept-Encoding, Access-Token, token, version")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, post, OPTIONS")
		c.Header("Access-Control-Max-Age", "1728000")
		c.Header("Access-Control-Allow-Credentials", "true")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}
		c.Next()
	}
}

func InstallAndTenant() gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/install") ||
			strings.HasPrefix(c.Request.URL.Path, "/crontab") ||
			c.Request.URL.Path == "/healthz" || c.Request.URL.Path == "/readyz" ||
			isStaticPath(c.Request.URL.Path) {
			c.Next()
			return
		}
		meta := ctxutil.Get(c)
		lock := config.C.App.InstallLock
		if lock != "" {
			if _, err := os.Stat(lock); err != nil {
				response.AbortFail(c, "程序未安装", response.CodeNotInstalled, 1)
				return
			}
		}
		first := firstSegment(c.Request.URL.Path)
		host := stripScheme(ctxutil.Host(c))
		if strings.Contains(first, "api") {
			if first == "platformapi" {
				meta.Source = ctxutil.SourcePlatform
				if tid := platformRequestTenantID(c); tid != "" {
					id := uint(atoi(tid))
					meta.TenantID = id
					bindPlatformTenant(meta, id)
				}
				c.Next()
				return
			}
			if !resolveTenant(c, meta, host, false) {
				return
			}
			c.Next()
			return
		}
		if first == "platform" {
			if config.C.Project.HTTPHost != "" && host != config.C.Project.HTTPHost {
				c.File(filepath.Join(config.C.App.PublicDir, "error", "platform", "404.html"))
				c.Abort()
				return
			}
			meta.Source = ctxutil.SourcePlatform
			if tid := platformRequestTenantID(c); tid != "" {
				id := uint(atoi(tid))
				meta.TenantID = id
				bindPlatformTenant(meta, id)
			}
			c.Next()
			return
		}
		if !resolveTenant(c, meta, host, true) {
			return
		}
		c.Next()
	}
}

func resolveTenant(c *gin.Context, meta *ctxutil.RequestMeta, host string, isPage bool) bool {
	if tenant, ok := tenantdb.ByHost(host); ok {
		if tenant.Disable == 0 && tenant.DomainAliasEnable == 0 {
			meta.TenantID = tenant.ID
			meta.TenantSN = tenant.SN
			meta.Tactics = tenant.Tactics
			return true
		}
		return tenantDisabled(c, isPage)
	}
	sn := ctxutil.SubDomain(host)
	meta.TenantSN = sn
	tenant, ok := tenantdb.BySN(sn)
	if !ok {
		return tenantMissing(c, isPage)
	}
	if tenant.Disable != 0 {
		return tenantDisabled(c, isPage)
	}
	meta.TenantID = tenant.ID
	meta.TenantSN = tenant.SN
	meta.Tactics = tenant.Tactics
	return true
}

func tenantDisabled(c *gin.Context, isPage bool) bool {
	if serveTenantError(c, isPage, "403.html") {
		return false
	}
	response.AbortFail(c, "该租户已停用", response.CodeForbidden, 0)
	return false
}

func tenantMissing(c *gin.Context, isPage bool) bool {
	if serveTenantError(c, isPage, "404.html") {
		return false
	}
	response.AbortFail(c, "接口域名错误或租户不存在", response.CodeNotFound, 0)
	return false
}

func serveTenantError(c *gin.Context, isPage bool, name string) bool {
	if !isPage {
		return false
	}
	c.File(filepath.Join(config.C.App.PublicDir, "error", "tenant", name))
	c.Abort()
	return true
}

func firstSegment(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	return strings.Split(path, "/")[0]
}

func isStaticPath(path string) bool {
	switch firstSegment(path) {
	case "resource", "uploads", "static":
		return true
	case "admin", "platform", "mobile", "pc":
		return isHashedAsset(path)
	}
	return false
}

func isHashedAsset(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".js", ".css", ".map", ".woff", ".woff2", ".ttf", ".eot", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp":
		return true
	}
	return false
}

func stripScheme(host string) string {
	host = strings.TrimPrefix(host, "http://")
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimSuffix(host, "/")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return host
}

func bindPlatformTenant(meta *ctxutil.RequestMeta, id uint) {
	if meta == nil || id == 0 {
		return
	}
	tenant, ok := tenantdb.ByID(id)
	if !ok {
		return
	}
	meta.TenantSN = tenant.SN
	meta.Tactics = tenant.Tactics
}

// platformRequestTenantID matches PHP LikeAdminAllowMiddleware: request()->param()
// so tenant_id/tenantId may arrive on the query string or in the JSON/form body
// (platform axios isParamsToData moves GET params onto POST body).
func platformRequestTenantID(c *gin.Context) string {
	if v := firstNonEmpty(c.Query("tenant_id"), c.Query("tenantId")); v != "" {
		return v
	}
	return firstNonEmpty(httpx.BodyStr(c, "tenant_id"), httpx.BodyStr(c, "tenantId"))
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}
