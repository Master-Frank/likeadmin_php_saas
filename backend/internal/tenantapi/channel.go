package tenantapi

import (
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/filesvc"
	"likeadmin/backend/internal/httpx"
	"likeadmin/backend/internal/response"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

func ChannelOAGet(c *gin.Context) {
	host := wechat.HostName(c)
	qr := cfgsvc.GetString(c, "oa_setting", "qr_code", "")
	if qr != "" {
		qr = filesvc.GetFileURL(c, qr)
	}
	enc := cfgsvc.GetInt(c, "oa_setting", "encryption_type", 1)
	response.Data(c, gin.H{
		"name":             cfgsvc.GetString(c, "oa_setting", "name", ""),
		"original_id":      cfgsvc.GetString(c, "oa_setting", "original_id", ""),
		"qr_code":          qr,
		"app_id":           cfgsvc.GetString(c, "oa_setting", "app_id", ""),
		"app_secret":       cfgsvc.GetString(c, "oa_setting", "app_secret", ""),
		"url":              ctxutil.Domain(c) + "/tenantapi/channel.official_account_reply/index",
		"token":            cfgsvc.GetString(c, "oa_setting", "token", ""),
		"encoding_aes_key": cfgsvc.GetString(c, "oa_setting", "encoding_aes_key", ""),
		"encryption_type":  enc,
		"business_domain":  host,
		"js_secure_domain": host,
		"web_auth_domain":  host,
	})
}

func ChannelOASet(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if msg := util.ChannelOASetCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "oa_setting", "name", httpx.BodyStr(c, "name"))
	cfgsvc.Set(c, "oa_setting", "original_id", httpx.BodyStr(c, "original_id"))
	cfgsvc.Set(c, "oa_setting", "qr_code", filesvc.SetFileURL(c, httpx.BodyStr(c, "qr_code")))
	cfgsvc.Set(c, "oa_setting", "app_id", httpx.BodyStr(c, "app_id"))
	cfgsvc.Set(c, "oa_setting", "app_secret", httpx.BodyStr(c, "app_secret"))
	cfgsvc.Set(c, "oa_setting", "token", httpx.BodyStr(c, "token"))
	cfgsvc.Set(c, "oa_setting", "encoding_aes_key", httpx.BodyStr(c, "encoding_aes_key"))
	cfgsvc.Set(c, "oa_setting", "encryption_type", httpx.BodyInt(c, "encryption_type"))
	response.SuccessNotice(c, "操作成功")
}

func ChannelMnpGet(c *gin.Context) {
	host := wechat.HostName(c)
	qr := cfgsvc.GetString(c, "mnp_setting", "qr_code", "")
	if qr != "" {
		qr = filesvc.GetFileURL(c, qr)
	}
	httpsHost := "https://" + host
	response.Data(c, gin.H{
		"name":                 cfgsvc.GetString(c, "mnp_setting", "name", ""),
		"original_id":          cfgsvc.GetString(c, "mnp_setting", "original_id", ""),
		"qr_code":              qr,
		"app_id":               cfgsvc.GetString(c, "mnp_setting", "app_id", ""),
		"app_secret":           cfgsvc.GetString(c, "mnp_setting", "app_secret", ""),
		"request_domain":       httpsHost,
		"socket_domain":        "wss://" + host,
		"upload_file_domain":   httpsHost,
		"download_file_domain": httpsHost,
		"udp_domain":           "udp://" + host,
		"business_domain":      host,
	})
}

func ChannelMnpSet(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if msg := util.ChannelMnpSetCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "mnp_setting", "name", httpx.BodyStr(c, "name"))
	cfgsvc.Set(c, "mnp_setting", "original_id", httpx.BodyStr(c, "original_id"))
	cfgsvc.Set(c, "mnp_setting", "qr_code", filesvc.SetFileURL(c, httpx.BodyStr(c, "qr_code")))
	cfgsvc.Set(c, "mnp_setting", "app_id", httpx.BodyStr(c, "app_id"))
	cfgsvc.Set(c, "mnp_setting", "app_secret", httpx.BodyStr(c, "app_secret"))
	response.SuccessNotice(c, "操作成功")
}

func ChannelOpenGet(c *gin.Context) {
	response.Data(c, gin.H{
		"app_id":     cfgsvc.GetString(c, "open_platform", "app_id", ""),
		"app_secret": cfgsvc.GetString(c, "open_platform", "app_secret", ""),
	})
}

func ChannelOpenSet(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if msg := util.ChannelOpenSetCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "open_platform", "app_id", httpx.BodyStr(c, "app_id"))
	cfgsvc.Set(c, "open_platform", "app_secret", httpx.BodyStr(c, "app_secret"))
	response.SuccessNotice(c, "操作成功")
}

func ChannelH5Get(c *gin.Context) {
	response.Data(c, gin.H{
		"status":      cfgsvc.GetInt(c, "web_page", "status", 1),
		"page_status": cfgsvc.GetInt(c, "web_page", "page_status", 0),
		"page_url":    cfgsvc.GetString(c, "web_page", "page_url", ""),
		"url":         ctxutil.Domain(c) + "/mobile",
	})
}

func ChannelH5Set(c *gin.Context) {
	if !response.RequirePOST(c) {
		return
	}
	if msg := util.ChannelH5SetCheck(httpx.Body(c)); msg != "" {
		response.Fail(c, msg)
		return
	}
	cfgsvc.Set(c, "web_page", "status", httpx.BodyInt(c, "status"))
	cfgsvc.Set(c, "web_page", "page_status", httpx.BodyInt(c, "page_status"))
	cfgsvc.Set(c, "web_page", "page_url", httpx.BodyStr(c, "page_url"))
	response.SuccessNotice(c, "操作成功")
}

func ChannelAppGet(c *gin.Context) {
	response.Data(c, gin.H{
		"ios_download_url":     cfgsvc.GetString(c, "app", "ios_download_url", ""),
		"android_download_url": cfgsvc.GetString(c, "app", "android_download_url", ""),
		"download_title":       cfgsvc.GetString(c, "app", "download_title", ""),
	})
}

func ChannelAppSet(c *gin.Context) {
	cfgsvc.Set(c, "app", "ios_download_url", httpx.BodyStr(c, "ios_download_url"))
	cfgsvc.Set(c, "app", "android_download_url", httpx.BodyStr(c, "android_download_url"))
	cfgsvc.Set(c, "app", "download_title", httpx.BodyStr(c, "download_title"))
	response.SuccessNotice(c, "操作成功")
}

func ChannelGetSet(group string) (gin.HandlerFunc, gin.HandlerFunc) {
	switch group {
	case "official_account", "oa_setting":
		return ChannelOAGet, ChannelOASet
	case "mnp", "mnp_setting":
		return ChannelMnpGet, ChannelMnpSet
	case "open", "open_platform":
		return ChannelOpenGet, ChannelOpenSet
	case "h5", "web_page":
		return ChannelH5Get, ChannelH5Set
	default:
		return ChannelAppGet, ChannelAppSet
	}
}

func ChannelGetOnly(group string) gin.HandlerFunc {
	g, _ := ChannelGetSet(group)
	return g
}

func ChannelSetOnly(group string) gin.HandlerFunc {
	_, s := ChannelGetSet(group)
	return s
}
