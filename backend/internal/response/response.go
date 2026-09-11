package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	CodeFail         = 0
	CodeOK           = 1
	CodeOpenNewPage  = 2
	CodeForbidden    = 3
	CodeNotFound     = 4
	CodeLoginExpire  = -1
	CodeNotInstalled = -2
)

type Body struct {
	Code int    `json:"code"`
	Show int    `json:"show"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

func emptyArray() any {
	return []any{}
}

func Result(c *gin.Context, code, show int, msg string, data any) {
	if data == nil {
		data = emptyArray()
	}
	c.JSON(http.StatusOK, Body{Code: code, Show: show, Msg: msg, Data: data})
}

func Success(c *gin.Context, msg string, data any) {
	if msg == "" {
		msg = "success"
	}
	if data == nil {
		data = emptyArray()
	}
	// Match BaseLikeAdminController::success default show=0
	Result(c, CodeOK, 0, msg, data)
}

// SuccessNotice matches PHP success($msg, [], 1, 1) used by most write actions.
func SuccessNotice(c *gin.Context, msg string) {
	if msg == "" {
		msg = "success"
	}
	Result(c, CodeOK, 1, msg, emptyArray())
}

func SuccessSilent(c *gin.Context, msg string, data any) {
	if data == nil {
		data = emptyArray()
	}
	Result(c, CodeOK, 0, msg, data)
}

func Data(c *gin.Context, data any) {
	if data == nil {
		data = map[string]any{}
	}
	Result(c, CodeOK, 0, "", data)
}

// DataCached sets a weak ETag and short public Cache-Control for public-read APIs.
func DataCached(c *gin.Context, data any, maxAge time.Duration) {
	if data == nil {
		data = map[string]any{}
	}
	raw, _ := json.Marshal(data)
	etag := `W/"` + util.MD5(string(raw)) + `"`
	if maxAge <= 0 {
		maxAge = 30 * time.Second
	}
	c.Header("ETag", etag)
	c.Header("Cache-Control", fmt.Sprintf("public, max-age=%d", int(maxAge.Seconds())))
	if match := c.GetHeader("If-None-Match"); match != "" && match == etag {
		c.AbortWithStatus(http.StatusNotModified)
		return
	}
	Data(c, data)
}

func Fail(c *gin.Context, msg string) {
	if msg == "" {
		msg = "fail"
	}
	Result(c, CodeFail, 1, msg, emptyArray())
}

// FailWithData matches PHP fail($msg, $data) used by pay/prepay retries.
func FailWithData(c *gin.Context, msg string, data any) {
	if msg == "" {
		msg = "fail"
	}
	if data == nil {
		data = emptyArray()
	}
	Result(c, CodeFail, 1, msg, data)
}

func FailSilent(c *gin.Context, msg string) {
	Result(c, CodeFail, 0, msg, emptyArray())
}

func FailCode(c *gin.Context, msg string, code, show int) {
	Result(c, code, show, msg, emptyArray())
}

func Lists(c *gin.Context, lists any, count int64, pageNo, pageSize int, extend any) {
	if lists == nil {
		lists = []any{}
	}
	if extend == nil {
		extend = []any{}
	}
	if ExportHook != nil && ExportHook(c, lists, count) {
		return
	}
	Data(c, gin.H{
		"lists":     lists,
		"count":     count,
		"page_no":   pageNo,
		"page_size": pageSize,
		"extend":    extend,
	})
}

// ExportHook is set by router to handle export=1/2 without import cycles.
var ExportHook func(c *gin.Context, rows any, count int64) bool

func AbortTooLarge(c *gin.Context) {
	if c == nil {
		return
	}
	c.Abort()
	c.JSON(http.StatusRequestEntityTooLarge, Body{
		Code: CodeFail, Show: 1, Msg: "请求体过大", Data: emptyArray(),
	})
}

func AbortFail(c *gin.Context, msg string, code, show int) {
	FailCode(c, msg, code, show)
	c.Abort()
}

// RequirePOST matches PHP BaseValidate::post(): reject non-POST before reading the body.
func RequirePOST(c *gin.Context) bool {
	if c != nil && c.Request != nil && c.Request.Method != http.MethodPost {
		Fail(c, "请求方式错误，请使用post请求方式")
		return false
	}
	return true
}
