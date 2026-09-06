package middleware

import (
	"net/http"
	"os"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Authorization, Sec-Fetch-Mode, DNT, X-Mx-ReqToken, Keep-Alive, User-Agent, If-Match, If-None-Match, If-Unmodified-Since, X-Requested-With, If-Modified-Since, Cache-Control, Content-Type, Accept-Language, Origin, Accept-Encoding, Access-Token, token, version")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
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
		if strings.HasPrefix(c.Request.URL.Path, "/install") {
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
				if tid := c.Query("tenant_id"); tid == "" {
					tid = c.Query("tenantId")
				}
				if tid := firstNonEmpty(c.Query("tenant_id"), c.Query("tenantId")); tid != "" {
					meta.TenantID = uint(atoi(tid))
				}
				c.Next()
				return
			}
			if !resolveTenant(c, meta, host) {
				return
			}
			c.Next()
			return
		}
		if first == "platform" {
			if config.C.Project.HTTPHost != "" && host != config.C.Project.HTTPHost {
				c.File(config.C.App.PublicDir + "/error/platform/404.html")
				c.Abort()
				return
			}
			meta.Source = ctxutil.SourcePlatform
			c.Next()
			return
		}
		if !resolveTenant(c, meta, host) {
			return
		}
		c.Next()
	}
}

func resolveTenant(c *gin.Context, meta *ctxutil.RequestMeta, host string) bool {
	var tenant model.Tenant
	err := bootstrap.DB.Where("domain_alias = ? AND delete_time IS NULL", host).First(&tenant).Error
	if err == nil {
		if tenant.Disable == 0 && tenant.DomainAliasEnable == 0 {
			meta.TenantID = tenant.ID
			meta.TenantSN = tenant.SN
			return true
		}
		response.AbortFail(c, "该租户已停用", response.CodeForbidden, 1)
		return false
	}
	sn := ctxutil.SubDomain(host)
	meta.TenantSN = sn
	err = bootstrap.DB.Where("sn = ? AND delete_time IS NULL", sn).First(&tenant).Error
	if err != nil {
		response.AbortFail(c, "接口域名错误或租户不存在", response.CodeNotFound, 1)
		return false
	}
	if tenant.Disable != 0 {
		response.AbortFail(c, "该租户已停用", response.CodeForbidden, 1)
		return false
	}
	meta.TenantID = tenant.ID
	meta.TenantSN = tenant.SN
	return true
}

func firstSegment(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}
	return strings.Split(path, "/")[0]
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
