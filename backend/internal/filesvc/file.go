package filesvc

import (
	"regexp"
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

// RewriteContentDomains prefixes relative img/video src with the file domain,
// matching PHP get_file_domain().
func RewriteContentDomains(c *gin.Context, content string) string {
	return rewriteContent(GetFileURL(c, ""), content)
}

var (
	imgSrcRe   = regexp.MustCompile(`(?is)(<img\s+[^>]*src=")([^"]*)(")`)
	videoSrcRe = regexp.MustCompile(`(?is)(<video\s+[^>]*src=")([^"]*)(")`)
)

func rewriteContent(fileURL, content string) string {
	if content == "" || fileURL == "" {
		return content
	}
	return videoSrcRe.ReplaceAllStringFunc(imgSrcRe.ReplaceAllStringFunc(content, func(m string) string {
		return prefixMediaSrc(fileURL, imgSrcRe, m)
	}), func(m string) string {
		return prefixMediaSrc(fileURL, videoSrcRe, m)
	})
}

func prefixMediaSrc(fileURL string, re *regexp.Regexp, match string) string {
	parts := re.FindStringSubmatch(match)
	if len(parts) != 4 {
		return match
	}
	src := parts[2]
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		return match
	}
	return parts[1] + fileURL + src + parts[3]
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
