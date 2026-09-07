package filesvc

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

// GetImageAttr matches PHP BaseModel::getImageAttr: empty stays "", else getFileUrl.
func GetImageAttr(c *gin.Context, uri string) string {
	if strings.TrimSpace(uri) == "" {
		return ""
	}
	return GetFileURL(c, uri)
}

func GetFileURL(c *gin.Context, uri string) string {
	if strings.Contains(uri, "http://") || strings.Contains(uri, "https://") {
		return uri
	}
	def := storageDefault(c)
	var domain string
	if def == "local" {
		domain = ctxutil.Domain(c)
	} else if engine := storageEngine(c, def); engine != nil {
		domain, _ = engine["domain"].(string)
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
	imgSrcSQRe = regexp.MustCompile(`(?is)(<img\s+[^>]*src=')([^']*)(')`)
	videoSrcRe = regexp.MustCompile(`(?is)(<video\s+[^>]*src=")([^"]*)(")`)
)

func rewriteContent(fileURL, content string) string {
	if content == "" || fileURL == "" {
		return content
	}
	return mapMediaSrc(content, func(src string) string {
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			return src
		}
		return fileURL + src
	})
}

// ClearContentDomains strips the file domain from img src only, matching PHP
// clear_file_domain() (video tags are left as-is; get_file_domain still rewrites both).
func ClearContentDomains(c *gin.Context, content string) string {
	if content == "" {
		return content
	}
	strip := func(src string) string { return SetFileURL(c, src) }
	content = imgSrcRe.ReplaceAllStringFunc(content, func(m string) string {
		return rewriteMediaSrc(imgSrcRe, m, strip)
	})
	return imgSrcSQRe.ReplaceAllStringFunc(content, func(m string) string {
		return rewriteMediaSrc(imgSrcSQRe, m, strip)
	})
}

func mapMediaSrc(content string, rewrite func(string) string) string {
	if content == "" || rewrite == nil {
		return content
	}
	return videoSrcRe.ReplaceAllStringFunc(imgSrcRe.ReplaceAllStringFunc(content, func(m string) string {
		return rewriteMediaSrc(imgSrcRe, m, rewrite)
	}), func(m string) string {
		return rewriteMediaSrc(videoSrcRe, m, rewrite)
	})
}

func rewriteMediaSrc(re *regexp.Regexp, match string, rewrite func(string) string) string {
	parts := re.FindStringSubmatch(match)
	if len(parts) != 4 {
		return match
	}
	return parts[1] + rewrite(parts[2]) + parts[3]
}

func SetFileURL(c *gin.Context, uri string) string {
	if uri == "" {
		return ""
	}
	def := storageDefault(c)
	var domain string
	if def == "local" {
		domain = ctxutil.Domain(c)
	} else if engine := storageEngine(c, def); engine != nil {
		domain, _ = engine["domain"].(string)
	}
	return strings.ReplaceAll(uri, strings.TrimRight(domain, "/")+"/", "")
}

func storageScope(c *gin.Context) uint {
	if c == nil {
		return 0
	}
	return ctxutil.Get(c).TenantID
}

func storageCacheKey(c *gin.Context, name string) string {
	if tid := storageScope(c); tid > 0 {
		return name + "_" + strconv.FormatUint(uint64(tid), 10)
	}
	return name
}

// ClearStorageCache drops tenant-scoped and legacy global storage URL keys.
func ClearStorageCache(c *gin.Context) {
	cache.Del(storageCacheKey(c, "STORAGE_DEFAULT"))
	cache.Del(storageCacheKey(c, "STORAGE_ENGINE"))
	cache.Del("STORAGE_DEFAULT")
	cache.Del("STORAGE_ENGINE")
}

func storageDefault(c *gin.Context) string {
	key := storageCacheKey(c, "STORAGE_DEFAULT")
	if raw, ok := cache.Get(key); ok && raw != "" {
		return raw
	}
	def := cfgsvc.GetString(c, "storage", "default", "local")
	if def != "" {
		cache.Set(key, def, 0)
	}
	return def
}

func storageEngine(c *gin.Context, def string) map[string]any {
	key := storageCacheKey(c, "STORAGE_ENGINE")
	var cached map[string]any
	if cache.GetJSON(key, &cached) && cached != nil {
		return cached
	}
	engine := cfgsvc.Get(c, "storage", def, nil)
	if m, ok := engine.(map[string]any); ok && m != nil {
		cache.Set(key, m, 0)
		return m
	}
	return nil
}

// PublicPath mirrors PHP FileService::getFileUrl($uri, 'public_path').
func PublicPath(uri string) string {
	root := strings.TrimRight(filepath.ToSlash(config.C.App.PublicDir), "/")
	uri = strings.TrimLeft(filepath.ToSlash(uri), "/")
	if root == "" {
		if uri == "" {
			return ""
		}
		return uri
	}
	if uri == "" {
		return root + "/"
	}
	return root + "/" + uri
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
