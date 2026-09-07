package middleware

import (
	"bytes"
	"encoding/json"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

// demoMaskURIs is PHP EncryDemoDataMiddleware::$needCheck after lower_uri().
var demoMaskURIs = map[string]bool{
	"setting.storage/detail":                   true,
	"notice.smsconfig/detail":                  true,
	"channel.officialaccountsetting/getconfig": true,
	"channel.mnpsettings/getconfig":            true,
	"channel.opensetting/getconfig":            true,
	"setting.pay.payconfig/getconfig":          true,
}

func demoMaskMatch(controller, action string) bool {
	uri := strings.ToLower(strings.Trim(controller+"/"+action, "/"))
	return demoMaskURIs[uri] || demoMaskURIs[util.LowerURI(uri)]
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
		if !demoMaskMatch(meta.Controller, meta.Action) {
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
			case []any:
				out[k] = maskDemoValue(child)
			default:
				out[k] = item
			}
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			switch child := item.(type) {
			case string:
				out[i] = "******"
			case map[string]any, []any:
				out[i] = maskDemoValue(child)
			default:
				out[i] = item
			}
		}
		return out
	default:
		return v
	}
}
