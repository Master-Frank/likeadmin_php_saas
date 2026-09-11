package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

type bodyWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)
	return w.ResponseWriter.Write(b)
}

var (
	oplogOnce sync.Once
	oplogCh   chan model.OperationLog
)

func oplogAsync() bool {
	v := os.Getenv("LIKEADMIN_OPLOG_ASYNC")
	return v == "1" || strings.EqualFold(v, "true")
}

func enqueueOplog(row model.OperationLog) {
	if bootstrap.DB == nil {
		return
	}
	if !oplogAsync() {
		_ = bootstrap.DB.Create(&row).Error
		return
	}
	oplogOnce.Do(func() {
		oplogCh = make(chan model.OperationLog, 256)
		go func() {
			for item := range oplogCh {
				if bootstrap.DB != nil {
					_ = bootstrap.DB.Create(&item).Error
				}
			}
		}()
	})
	select {
	case oplogCh <- row:
	default:
		// drop low-value logs when the queue is full
	}
}

func OperationLog() gin.HandlerFunc {
	return func(c *gin.Context) {
		meta := ctxutil.Get(c)
		if meta.App != "platformapi" && meta.App != "tenantapi" {
			c.Next()
			return
		}
		if strings.EqualFold(meta.Controller, "setting.system.log") {
			c.Next()
			return
		}
		if !meta.NotNeedLogin && meta.AdminInfo == nil {
			c.Next()
			return
		}
		bw := &bodyWriter{ResponseWriter: c.Writer}
		c.Writer = bw
		c.Next()
		if bootstrap.DB == nil {
			return
		}
		params := httpx.Params(c)
		safe := make(map[string]any, len(params))
		for k, v := range params {
			safe[k] = v
		}
		for _, key := range []string{"password", "password_old", "old_password", "app_secret", "secret_key"} {
			if _, ok := safe[key]; ok {
				safe[key] = "******"
			}
		}
		raw, _ := json.Marshal(safe)
		action := ActionNotes(meta.Controller, meta.Action)
		if util.ToInt(params["export"]) == 2 {
			action += "-数据导出"
		}
		result := bw.buf.String()
		if requestLogType(c) == "GET" {
			result = ""
		} else if len(result) > 65535 {
			result = result[:65535]
		}
		adminID := meta.AdminID
		name, account := "", ""
		if meta.AdminInfo != nil {
			name = util.ToString(meta.AdminInfo["name"])
			account = util.ToString(meta.AdminInfo["account"])
		}
		row := model.OperationLog{
			AdminID: adminID, AdminName: name, Account: account,
			Action: action, Type: requestLogType(c), URL: requestAbsoluteURL(c),
			Params: string(raw), Result: result, IP: ctxutil.ClientIP(c),
			CreateTime: util.NowUnix(),
		}
		enqueueOplog(row)
	}
}

func requestLogType(c *gin.Context) string {
	if c != nil && c.Request != nil && (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) {
		return "GET"
	}
	return "POST"
}

// requestAbsoluteURL matches PHP $request->url(true): scheme://host + REQUEST_URI.
func requestAbsoluteURL(c *gin.Context) string {
	path := ""
	if c != nil && c.Request != nil {
		if c.Request.URL != nil {
			path = c.Request.URL.RequestURI()
			if path == "" {
				path = c.Request.URL.Path
				if c.Request.URL.RawQuery != "" {
					path += "?" + c.Request.URL.RawQuery
				}
			}
		}
		if path == "" {
			path = c.Request.RequestURI
		}
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return ctxutil.Domain(c) + path
}

func ReadBody(c *gin.Context) []byte {
	if v, ok := c.Get("likeadmin.raw"); ok {
		if b, ok := v.([]byte); ok {
			return b
		}
	}
	if c.Request.Body == nil {
		return nil
	}
	raw, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
	c.Set("likeadmin.raw", raw)
	return raw
}
