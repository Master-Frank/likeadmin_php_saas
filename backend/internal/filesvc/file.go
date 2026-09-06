package filesvc

import (
	"strings"

	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

func GetFileURL(c *gin.Context, uri string) string {
	if strings.Contains(uri, "http://") || strings.Contains(uri, "https://") {
		return uri
	}
	def := cfgsvc.GetString(c, "storage", "default", "local")
	var domain string
	if def == "local" {
		domain = ctxutil.Domain(c)
	} else {
		engine := cfgsvc.Get(c, "storage", def, nil)
		if m, ok := engine.(map[string]any); ok {
			domain, _ = m["domain"].(string)
		}
	}
	return Format(domain, uri)
}

func SetFileURL(c *gin.Context, uri string) string {
	if uri == "" {
		return ""
	}
	def := cfgsvc.GetString(c, "storage", "default", "local")
	var domain string
	if def == "local" {
		domain = ctxutil.Domain(c)
	} else {
		engine := cfgsvc.Get(c, "storage", def, nil)
		if m, ok := engine.(map[string]any); ok {
			domain, _ = m["domain"].(string)
		}
	}
	return strings.ReplaceAll(uri, strings.TrimRight(domain, "/")+"/", "")
}

func Format(domain, uri string) string {
	domain = strings.TrimRight(domain, "/")
	uri = strings.TrimLeft(uri, "/")
	if domain == "" {
		return uri
	}
	if uri == "" {
		return domain + "/"
	}
	return domain + "/" + uri
}
