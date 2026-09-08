package httpx

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

// Query mirrors PHP BaseValidate::$method=GET / request()->get():
// only the query string, never the JSON/form body.
func Query(c *gin.Context) map[string]any {
	if v, ok := c.Get("likeadmin.query"); ok {
		return v.(map[string]any)
	}
	out := map[string]any{}
	if c != nil && c.Request != nil {
		for k, vs := range c.Request.URL.Query() {
			if len(vs) == 1 {
				out[k] = vs[0]
			} else if len(vs) > 1 {
				out[k] = vs
			}
		}
	}
	if c != nil {
		c.Set("likeadmin.query", out)
	}
	return out
}

func QueryStr(c *gin.Context, key string) string {
	return strings.TrimSpace(util.ToString(Query(c)[key]))
}

func QueryInt(c *gin.Context, key string) int {
	return util.ToInt(Query(c)[key])
}

func QueryUint(c *gin.Context, key string) uint {
	return uint(util.ToInt(Query(c)[key]))
}

// QueryPresent reports whether the query string includes a key whose
// value ThinkPHP Validate "require" would accept. require treats a
// missing/empty string as absent but treats the literal "0" as present
// (!empty($value) || '0' == $value).
func QueryPresent(c *gin.Context, key string) bool {
	return util.PHPRequired(Query(c), key)
}

func QueryIDPresent(c *gin.Context) bool {
	return QueryPresent(c, "id")
}

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
	for k, v := range Body(c) {
		out[k] = v
	}
	c.Set("likeadmin.params", out)
	return out
}

// Body mirrors PHP request()->post(): JSON/form only, never the query string.
func Body(c *gin.Context) map[string]any {
	if v, ok := c.Get("likeadmin.body"); ok {
		return v.(map[string]any)
	}
	out := map[string]any{}
	if c != nil && c.Request != nil && c.Request.Body != nil {
		raw, _ := io.ReadAll(c.Request.Body)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(raw))
		c.Set("likeadmin.raw", raw)
		if len(raw) > 0 {
			var body map[string]any
			if json.Unmarshal(raw, &body) == nil {
				for k, v := range body {
					out[k] = v
				}
			} else {
				var arr []any
				if json.Unmarshal(raw, &arr) == nil {
					out["_list"] = arr
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
	}
	if c != nil {
		c.Set("likeadmin.body", out)
	}
	return out
}

func BodyStr(c *gin.Context, key string) string {
	return strings.TrimSpace(util.ToString(Body(c)[key]))
}

// BodyRaw matches ThinkPHP request()->post() scalars: no TrimSpace.
func BodyRaw(c *gin.Context, key string) string {
	return util.ToString(Body(c)[key])
}

func BodyInt(c *gin.Context, key string) int {
	return util.ToInt(Body(c)[key])
}

func BodyUint(c *gin.Context, key string) uint {
	return uint(util.ToInt(Body(c)[key]))
}

func BodyFloat(c *gin.Context, key string) float64 {
	return util.ToFloat(Body(c)[key])
}

func BodyAny(c *gin.Context, key string) any {
	return Body(c)[key]
}

func BodyHas(c *gin.Context, key string) bool {
	_, ok := Body(c)[key]
	return ok
}

// BodyPresent reports whether the JSON/form body includes a key whose
// value ThinkPHP Validate "require" would accept. Missing/empty string
// fail; the literal 0/"0" is present.
func BodyPresent(c *gin.Context, key string) bool {
	return util.PHPRequired(Body(c), key)
}

func BodyIDPresent(c *gin.Context) bool {
	return BodyPresent(c, "id")
}

func BodyInts(c *gin.Context, key string) []int {
	v := Body(c)[key]
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

func BodyUints(c *gin.Context, key string) []uint {
	ints := BodyInts(c, key)
	out := make([]uint, len(ints))
	for i, n := range ints {
		out[i] = uint(n)
	}
	return out
}

func List(c *gin.Context) []any {
	v := Body(c)["_list"]
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}

func Float(c *gin.Context, key string) float64 {
	return util.ToFloat(Params(c)[key])
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

// ParamTenantID matches PHP request()->param('tenant_id') / tenantId:
// query then body (body wins). Missing key => present=false.
func ParamTenantID(c *gin.Context) (id uint, present bool) {
	p := Params(c)
	if v, ok := p["tenant_id"]; ok && v != nil {
		return Uint(c, "tenant_id"), true
	}
	if v, ok := p["tenantId"]; ok && v != nil {
		return uint(util.ToInt(v)), true
	}
	return 0, false
}
