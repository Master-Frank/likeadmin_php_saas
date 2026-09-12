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

// GetImageAttr matches PHP BaseModel::getImageAttr / User::getAvatarAttr:
// trim($value) ? getFileUrl($value) : ”. "0" is falsy after trim.
func GetImageAttr(c *gin.Context, uri string) string {
	if t := strings.TrimSpace(uri); t == "" || t == "0" {
		return ""
	}
	return GetFileURL(c, uri)
}

// EmptyFileURL matches PayConfig/TenantPayConfig getIconAttr and
// CustomerServiceLogic: empty($value) ? ” : getFileUrl($value).
// No trim; "0" is empty.
func EmptyFileURL(c *gin.Context, uri string) string {
	if uri == "" || uri == "0" {
		return ""
	}
	return GetFileURL(c, uri)
}

// FileURLUnlessEmpty matches `empty($uri) ? $uri : getFileUrl($uri)`
// (OA/MNP qr_code, decorate tabbar icons). "" and "0" stay as stored.
func FileURLUnlessEmpty(c *gin.Context, uri string) string {
	if uri == "" || uri == "0" {
		return uri
	}
	return GetFileURL(c, uri)
}

// AdminAvatarURL matches Admin/TenantAdmin/Tenant getAvatarAttr:
// empty($value) ? getFileUrl($fallback) : getFileUrl(trim($value, '/')).
func AdminAvatarURL(c *gin.Context, stored, fallback string) string {
	if stored == "" || stored == "0" {
		stored = fallback
	} else {
		stored = strings.Trim(stored, "/")
	}
	return GetFileURL(c, stored)
}

// LoginUserAvatarURL matches LoginLogic: getter then `$avatar ?: default`
// then getFileUrl. Trim-falsy stored values (including "0") fall back.
func LoginUserAvatarURL(c *gin.Context, stored, fallback string) string {
	if got := GetImageAttr(c, stored); got != "" {
		return got
	}
	return GetFileURL(c, fallback)
}

func GetFileURL(c *gin.Context, uri string) string {
	if strings.Contains(uri, "http://") || strings.Contains(uri, "https://") {
		return uri
	}
	def := storageDefault(c)
	var domain string
	if def == "local" {
		domain = localFileDomain(c)
	} else if engine := storageEngine(c, def); engine != nil {
		domain, _ = engine["domain"].(string)
	}
	return Format(domain, uri)
}

func localFileDomain(c *gin.Context) string {
	if d := config.FileCDNDomain(); d != "" {
		return d
	}
	return ctxutil.Domain(c)
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
	// PHP clear_file_domain only strips img src that start with the current
	// FileService::getFileUrl() prefix. Keep other tag attrs (Go-ahead).
	base := strings.TrimRight(GetFileURL(c, ""), "/") + "/"
	if base == "/" {
		return content
	}
	strip := func(src string) string {
		if strings.HasPrefix(src, base) {
			return strings.TrimPrefix(src, base)
		}
		return src
	}
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

// SetImageIf matches `$value ? FileService::setFileUrl($value) : ”`
// (ArticleLogic image). Missing, "", "0" become ""; whitespace is kept.
func SetImageIf(c *gin.Context, uri string) string {
	if uri == "" || uri == "0" {
		return ""
	}
	return SetFileURL(c, uri)
}

// SetImageAttr matches PHP BaseModel::setImageAttr:
// trim($value) ? setFileUrl($value) : ”. The original (untrimmed) value
// is passed to setFileUrl when the trimmed form is truthy.
func SetImageAttr(c *gin.Context, uri string) string {
	if t := strings.TrimSpace(uri); t == "" || t == "0" {
		return ""
	}
	return SetFileURL(c, uri)
}

func SetFileURL(c *gin.Context, uri string) string {
	if uri == "" {
		return ""
	}
	def := storageDefault(c)
	var domain string
	if def == "local" {
		domain = localFileDomain(c)
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
		return "/" + uri
	}
	if uri == "" {
		return domain + "/"
	}
	return domain + "/" + uri
}
