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
	"likeadmin/backend/internal/schemacache"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const oplogCaptureCap = 65535

type bodyWriter struct {
	gin.ResponseWriter
	buf       bytes.Buffer
	cap       int
	truncated bool
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	if w.cap > 0 {
		remain := w.cap - w.buf.Len()
		if remain > 0 {
			if len(b) > remain {
				w.buf.Write(b[:remain])
				w.truncated = true
			} else {
				w.buf.Write(b)
			}
		} else {
			w.truncated = true
		}
	}
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
	write := func(item model.OperationLog) {
		if bootstrap.DB == nil {
			return
		}
		db := bootstrap.DB
		if !schemacache.HasColumn(db, item.TableName(), "tenant_id") {
			_ = db.Omit("tenant_id").Create(&item).Error
			return
		}
		_ = db.Create(&item).Error
	}
	if !oplogAsync() {
		write(row)
		return
	}
	oplogOnce.Do(func() {
		oplogCh = make(chan model.OperationLog, 256)
		go func() {
			for item := range oplogCh {
				write(item)
			}
		}()
	})
	select {
	case oplogCh <- row:
	default:
		// drop low-value logs when the queue is full
	}
}

func skipOplogCapture(c *gin.Context, meta *ctxutil.RequestMeta) bool {
	if requestLogType(c) == "GET" {
		return true
	}
	if meta != nil && strings.EqualFold(meta.Controller, "download") {
		return true
	}
	return false
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
		var bw *bodyWriter
		if !skipOplogCapture(c, meta) {
			bw = &bodyWriter{ResponseWriter: c.Writer, cap: oplogCaptureCap}
			c.Writer = bw
		}
		c.Next()
		if bootstrap.DB == nil {
			return
		}
		params := httpx.Params(c)
		raw, _ := json.Marshal(redactParams(params))
		action := ActionNotes(meta.Controller, meta.Action)
		if util.ToInt(params["export"]) == 2 {
			action += "-数据导出"
		}
		result := ""
		if bw != nil {
			result = bw.buf.String()
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
			TenantID: meta.TenantID, CreateTime: util.NowUnix(),
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

func redactParams(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			if credentialKey(k) {
				out[k] = "******"
				continue
			}
			out[k] = redactParams(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = redactParams(item)
		}
		return out
	case []map[string]any:
		out := make([]any, len(t))
		for i, item := range t {
			out[i] = redactParams(item)
		}
		return out
	default:
		return v
	}
}

func credentialKey(key string) bool {
	n := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "-", "_"))
	switch n {
	case "password", "password_old", "old_password", "password_confirm", "new_password",
		"app_secret", "secret_key", "secret", "private_key", "mch_key", "mch_secret",
		"access_key_secret", "access_key", "accesskeysecret", "api_key", "apikey",
		"aes_key", "encoding_aes_key", "token", "refresh_token", "app_key",
		"cert", "certificate", "cert_key", "key_pem", "client_secret":
		return true
	}
	for _, part := range []string{"password", "secret", "private_key", "access_key_secret", "aes_key"} {
		if strings.Contains(n, part) {
			return true
		}
	}
	return false
}
