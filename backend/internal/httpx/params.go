package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func Params(c *gin.Context) map[string]any {
	if v, ok := c.Get("likeadmin.params"); ok {
		return v.(map[string]any)
	}
	out := map[string]any{}
	for k, vs := range c.Request.URL.Query() {
		if len(vs) == 1 {
			out[k] = vs[0]
		} else if len(vs) > 1 {
			out[k] = vs
		}
	}
	if c.Request.Body != nil {
		raw, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		if len(raw) > 0 {
			var body map[string]any
			if json.Unmarshal(raw, &body) == nil {
				for k, v := range body {
					out[k] = v
				}
			} else {
				_ = c.Request.ParseForm()
				for k, vs := range c.Request.PostForm {
					if len(vs) == 1 {
						out[k] = vs[0]
					} else {
						out[k] = vs
					}
				}
			}
		}
	}
	c.Set("likeadmin.params", out)
	return out
}

func Str(c *gin.Context, key string) string {
	return strings.TrimSpace(util.ToString(Params(c)[key]))
}

func Int(c *gin.Context, key string) int {
	return util.ToInt(Params(c)[key])
}

func Uint(c *gin.Context, key string) uint {
	return uint(util.ToInt(Params(c)[key]))
}

func Any(c *gin.Context, key string) any {
	return Params(c)[key]
}

func Ints(c *gin.Context, key string) []int {
	v := Params(c)[key]
	switch t := v.(type) {
	case []any:
		out := make([]int, 0, len(t))
		for _, item := range t {
			out = append(out, util.ToInt(item))
		}
		return out
	case []int:
		return t
	default:
		if util.ToString(v) == "" {
			return nil
		}
		return []int{util.ToInt(v)}
	}
}

func Uints(c *gin.Context, key string) []uint {
	ints := Ints(c, key)
	out := make([]uint, len(ints))
	for i, n := range ints {
		out[i] = uint(n)
	}
	return out
}
