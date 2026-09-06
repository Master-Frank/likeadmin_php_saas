package middleware

import (
	"bytes"
	"encoding/json"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/response"

	"github.com/gin-gonic/gin"
)

var demoMaskURIs = map[string]bool{
	"setting.storage/detail":                     true,
	"notice.smsconfig/detail":                    true,
	"notice.sms_config/detail":                   true,
	"channel.official_account_setting/getconfig": true,
	"channel.mnp_settings/getconfig":             true,
	"channel.open_setting/getconfig":             true,
	"setting.pay.pay_config/getconfig":           true,
}

var demoMaskExclude = map[string]bool{
	"name": true, "icon": true, "image": true, "qr_code": true,
	"interface_version": true, "merchant_type": true,
}

type maskWriter struct {
	gin.ResponseWriter
	buf bytes.Buffer
}

func (w *maskWriter) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *maskWriter) WriteString(s string) (int, error) {
	return w.buf.WriteString(s)
}

func DemoMask() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.C.Project.DemoEnv {
			c.Next()
			return
		}
		meta := ctxutil.Get(c)
		uri := strings.ToLower(meta.Controller + "/" + meta.Action)
		if !demoMaskURIs[uri] {
			c.Next()
			return
		}
		mw := &maskWriter{ResponseWriter: c.Writer}
		c.Writer = mw
		c.Next()
		raw := mw.buf.Bytes()
		var body response.Body
		if json.Unmarshal(raw, &body) != nil {
			_, _ = mw.ResponseWriter.Write(raw)
			return
		}
		body.Data = maskDemoValue(body.Data)
		out, err := json.Marshal(body)
		if err != nil {
			_, _ = mw.ResponseWriter.Write(raw)
			return
		}
		_, _ = mw.ResponseWriter.Write(out)
	}
}

func maskDemoValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, item := range t {
			if demoMaskExclude[k] {
				out[k] = item
				continue
			}
			switch child := item.(type) {
			case string:
				out[k] = "******"
			case map[string]any:
				out[k] = maskDemoValue(child)
			default:
				out[k] = item
			}
		}
		return out
	default:
		return v
	}
}
