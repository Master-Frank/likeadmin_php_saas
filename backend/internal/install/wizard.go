package install

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

//go:embed wizardassets
var wizardAssets embed.FS

func Wizard(c *gin.Context) {
	// PHP install.php die()s this exact string when install.lock exists.
	if lock := config.C.App.InstallLock; lock != "" {
		if _, err := os.Stat(lock); err == nil {
			c.String(http.StatusOK, installedMsg)
			return
		}
	}
	data, err := fs.ReadFile(wizardAssets, "wizardassets/index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "安装向导资源缺失")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// Asset serves the original installer CSS/images from the Go binary.
// nginx location /install is proxied to Go, so these cannot live in public/.
func Asset(c *gin.Context) {
	name := strings.TrimPrefix(c.Param("filepath"), "/")
	name = path.Clean("/" + name)
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." || name == "index.html" || strings.Contains(name, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := fs.ReadFile(wizardAssets, path.Join("wizardassets", name))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctype := mime.TypeByExtension(path.Ext(name))
	if ctype == "" {
		switch path.Ext(name) {
		case ".css":
			ctype = "text/css; charset=utf-8"
		case ".png":
			ctype = "image/png"
		case ".ico":
			ctype = "image/x-icon"
		default:
			ctype = "application/octet-stream"
		}
	}
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, ctype, data)
}
