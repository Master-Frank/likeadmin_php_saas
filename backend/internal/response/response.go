package response

import (
	"net/http"

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

func Fail(c *gin.Context, msg string) {
	if msg == "" {
		msg = "fail"
	}
	Result(c, CodeFail, 1, msg, emptyArray())
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

func AbortFail(c *gin.Context, msg string, code, show int) {
	FailCode(c, msg, code, show)
	c.Abort()
}
