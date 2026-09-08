package cron

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"

	"likeadmin/backend/internal/config"

	"github.com/gin-gonic/gin"
)

const crontabTokenEnv = "LIKEADMIN_CRONTAB_TOKEN"

// HTTP runs one scheduler pass. PHP exposed GET /crontab for wget; keep that
// for debug and optional token auth. Production should use cmd/crontab.
func HTTP(c *gin.Context) {
	if !HTTPAllowed(c.GetHeader("X-Crontab-Token"), c.Query("token")) {
		c.String(http.StatusUnauthorized, "forbidden")
		return
	}
	RunOnce()
	c.String(http.StatusOK, "ok")
}

// HTTPAllowed is true when no token is configured and debug is on (pair.sh),
// or the request presents LIKEADMIN_CRONTAB_TOKEN. Non-debug deploys without
// a token must use the worker, not the public HTTP trigger.
func HTTPAllowed(header, query string) bool {
	want := strings.TrimSpace(os.Getenv(crontabTokenEnv))
	if want != "" {
		got := strings.TrimSpace(header)
		if got == "" {
			got = strings.TrimSpace(query)
		}
		if len(got) != len(want) {
			return false
		}
		return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
	}
	return config.C.App.Debug
}
