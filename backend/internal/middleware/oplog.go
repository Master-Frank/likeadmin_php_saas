package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

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
		action := meta.Controller + "/" + meta.Action
		if util.ToInt(params["export"]) == 2 {
			action += "-数据导出"
		}
		result := bw.buf.String()
		if len(result) > 4000 {
			result = result[:4000]
		}
		adminID := meta.AdminID
		name, account := "", ""
		if meta.AdminInfo != nil {
			name = util.ToString(meta.AdminInfo["name"])
			account = util.ToString(meta.AdminInfo["account"])
		}
		row := model.OperationLog{
			AdminID: adminID, AdminName: name, Account: account,
			Action: action, Type: c.Request.Method, URL: c.Request.URL.String(),
			Params: string(raw), Result: result, IP: ctxutil.ClientIP(c),
			CreateTime: util.NowUnix(),
		}
		_ = bootstrap.DB.Create(&row).Error
	}
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
