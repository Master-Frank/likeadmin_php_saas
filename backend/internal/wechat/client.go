package wechat

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"likeadmin/backend/internal/cache"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	TerminalMNP     = 1
	TerminalOA      = 2
	TerminalH5      = 3
	TerminalPC      = 4
	TerminalIOS     = 5
	TerminalAndroid = 6
)

type Session struct {
	Openid      string `json:"openid"`
	Unionid     string `json:"unionid"`
	SessionKey  string `json:"session_key"`
	AccessToken string `json:"access_token"`
	Nickname    string `json:"nickname"`
	Headimgurl  string `json:"headimgurl"`
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
}

var httpClient = &http.Client{Timeout: 8 * time.Second}

func OAConfig(c *gin.Context) (appID, secret, token string) {
	return cfgsvc.GetString(c, "oa_setting", "app_id", ""),
		cfgsvc.GetString(c, "oa_setting", "app_secret", ""),
		cfgsvc.GetString(c, "oa_setting", "token", "")
}

func MnpConfig(c *gin.Context) (appID, secret string) {
	return cfgsvc.GetString(c, "mnp_setting", "app_id", ""),
		cfgsvc.GetString(c, "mnp_setting", "app_secret", "")
}

func OpenConfig(c *gin.Context) (appID, secret string) {
	return cfgsvc.GetString(c, "open_platform", "app_id", ""),
		cfgsvc.GetString(c, "open_platform", "app_secret", "")
}

func Code2Session(appID, secret, code string) (Session, error) {
	q := url.Values{}
	q.Set("appid", appID)
	q.Set("secret", secret)
	q.Set("js_code", code)
	q.Set("grant_type", "authorization_code")
	var s Session
	if err := getJSON("https://api.weixin.qq.com/sns/jscode2session?"+q.Encode(), &s); err != nil {
		return s, err
	}
	if s.Openid == "" {
		// PHP WeChatMnpService::getMnpResByCode always throws this when openid is empty.
		return s, fmt.Errorf("获取openID失败")
	}
	return s, nil
}

func OAuthByCode(appID, secret, code string) (Session, error) {
	q := url.Values{}
	q.Set("appid", appID)
	q.Set("secret", secret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")
	var tok struct {
		AccessToken string `json:"access_token"`
		Openid      string `json:"openid"`
		Unionid     string `json:"unionid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := getJSON("https://api.weixin.qq.com/sns/oauth2/access_token?"+q.Encode(), &tok); err != nil {
		return Session{}, err
	}
	s := Session{Openid: tok.Openid, Unionid: tok.Unionid, AccessToken: tok.AccessToken}
	if tok.Openid == "" {
		// PHP WeChatOaService::getOaResByCode always throws this when openid is empty.
		return s, fmt.Errorf("获取openID失败")
	}
	if tok.AccessToken != "" {
		var info Session
		_ = getJSON("https://api.weixin.qq.com/sns/userinfo?access_token="+url.QueryEscape(tok.AccessToken)+
			"&openid="+url.QueryEscape(tok.Openid)+"&lang=zh_CN", &info)
		if info.Nickname != "" {
			s.Nickname = info.Nickname
		}
		if info.Headimgurl != "" {
			s.Headimgurl = info.Headimgurl
		}
		if info.Unionid != "" {
			s.Unionid = info.Unionid
		}
	}
	return s, nil
}

func CodeURL(appID, redirect string) string {
	q := url.Values{}
	q.Set("appid", appID)
	q.Set("redirect_uri", redirect)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_userinfo")
	q.Set("state", "likeadmin")
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + q.Encode() + "#wechat_redirect"
}

func ScanCodeURL(appID, redirect, state string) string {
	q := url.Values{}
	q.Set("appid", appID)
	q.Set("redirect_uri", redirect)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_login")
	q.Set("state", state)
	return "https://open.weixin.qq.com/connect/qrconnect?" + q.Encode() + "#wechat_redirect"
}

func accessTokenCacheKey(appID, secret string) string {
	// EasyWeChat keys official_account.access_token.{appId}.{secret} so a
	// rotated secret cannot reuse a cached token. Hash the secret so Redis
	// KEYS does not echo the raw value.
	return "wechat_access_token_" + appID + "_" + util.MD5(secret)
}

func AccessToken(appID, secret string) (string, error) {
	key := accessTokenCacheKey(appID, secret)
	if v, ok := cache.Get(key); ok && v != "" {
		return v, nil
	}
	q := url.Values{}
	q.Set("grant_type", "client_credential")
	q.Set("appid", appID)
	q.Set("secret", secret)
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := getJSON("https://api.weixin.qq.com/cgi-bin/token?"+q.Encode(), &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", fmt.Errorf(firstNonEmpty(out.ErrMsg, "获取access_token失败"))
	}
	if ttl, ok := wechatCacheTTL(out.ExpiresIn); ok {
		cache.Set(key, out.AccessToken, ttl)
	}
	return out.AccessToken, nil
}

// wechatCacheTTL mirrors EasyWeChat: cache expires_in-200s, skip when expires_in<=200.
func wechatCacheTTL(expiresIn int) (time.Duration, bool) {
	if expiresIn <= 200 {
		return 0, false
	}
	return time.Duration(expiresIn-200) * time.Second, true
}

// DefaultJSApiList matches PHP WechatLogic::jsConfig / EasyWeChat buildJsSdkConfig.
var DefaultJSApiList = []string{
	"onMenuShareTimeline",
	"onMenuShareAppMessage",
	"onMenuShareQQ",
	"onMenuShareWeibo",
	"onMenuShareQZone",
	"openLocation",
	"getLocation",
	"chooseWXPay",
	"updateAppMessageShareData",
	"updateTimelineShareData",
	"openAddress",
	"scanQRCode",
}

func jsSDKConfig(appID string, ts int64, nonce, signature, pageURL string) map[string]any {
	// PHP EasyWeChat JsApiTicket::configSignature includes url.
	return map[string]any{
		"url":         pageURL,
		"appId":       appID,
		"timestamp":   ts,
		"nonceStr":    nonce,
		"signature":   signature,
		"jsApiList":   DefaultJSApiList,
		"openTagList": []string{},
		"debug":       false,
	}
}

func JsConfig(appID, secret, rawURL string) (map[string]any, error) {
	ticket, err := jsapiTicket(appID, secret)
	if err != nil {
		return nil, err
	}
	ts := util.NowUnix()
	nonce := util.MD5(fmt.Sprintf("%d", ts))
	u := rawURL
	if dec, err := url.QueryUnescape(rawURL); err == nil && dec != "" {
		u = dec
	}
	signSrc := fmt.Sprintf("jsapi_ticket=%s&noncestr=%s&timestamp=%d&url=%s", ticket, nonce, ts, u)
	sum := sha1.Sum([]byte(signSrc))
	return jsSDKConfig(appID, ts, nonce, hex.EncodeToString(sum[:]), u), nil
}

func jsapiTicket(appID, secret string) (string, error) {
	key := "wechat_jsapi_ticket_" + appID
	if v, ok := cache.Get(key); ok && v != "" {
		return v, nil
	}
	tok, err := AccessToken(appID, secret)
	if err != nil {
		return "", err
	}
	var out struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
	}
	if err := getJSON("https://api.weixin.qq.com/cgi-bin/ticket/getticket?access_token="+url.QueryEscape(tok)+"&type=jsapi", &out); err != nil {
		return "", err
	}
	if out.Ticket == "" {
		return "", fmt.Errorf(firstNonEmpty(out.ErrMsg, "获取jsapi_ticket失败"))
	}
	if ttl, ok := wechatCacheTTL(out.ExpiresIn); ok {
		cache.Set(key, out.Ticket, ttl)
	}
	return out.Ticket, nil
}

func PhoneNumber(appID, secret, code string) (string, error) {
	tok, err := AccessToken(appID, secret)
	if err != nil {
		return "", err
	}
	body := map[string]any{"code": code}
	var out struct {
		ErrCode   int `json:"errcode"`
		ErrMsg    string
		PhoneInfo struct {
			PurePhoneNumber string `json:"purePhoneNumber"`
			PhoneNumber     string `json:"phoneNumber"`
		} `json:"phone_info"`
	}
	if err := postJSON("https://api.weixin.qq.com/wxa/business/getuserphonenumber?access_token="+url.QueryEscape(tok), body, &out); err != nil {
		return "", err
	}
	phone := out.PhoneInfo.PurePhoneNumber
	if phone == "" {
		phone = out.PhoneInfo.PhoneNumber
	}
	if phone == "" {
		return "", fmt.Errorf(firstNonEmpty(out.ErrMsg, "获取手机号码失败"))
	}
	return phone, nil
}

func PublishMenu(appID, secret string, buttons []any) error {
	tok, err := AccessToken(appID, secret)
	if err != nil {
		return err
	}
	var out struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	raw, err := postJSONBytes("https://api.weixin.qq.com/cgi-bin/menu/create?access_token="+url.QueryEscape(tok),
		map[string]any{"button": BuildMenuButtons(buttons)}, &out)
	if err != nil {
		return err
	}
	if out.ErrCode != 0 {
		// PHP OfficialAccountMenuLogic::saveAndPublish:
		// '保存发布菜单失败' . json_encode($result->getContent())
		quoted, _ := json.Marshal(string(raw))
		return fmt.Errorf("保存发布菜单失败%s", quoted)
	}
	return nil
}

func CheckOASignature(token, signature, timestamp, nonce string) bool {
	if token == "" || signature == "" {
		return token == ""
	}
	arr := []string{token, timestamp, nonce}
	sort.Strings(arr)
	sum := sha1.Sum([]byte(strings.Join(arr, "")))
	return hex.EncodeToString(sum[:]) == strings.ToLower(signature)
}

func getJSON(rawURL string, dest any) error {
	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func postJSON(rawURL string, body any, dest any) error {
	_, err := postJSONBytes(rawURL, body, dest)
	return err
}

func postJSONBytes(rawURL string, body any, dest any) ([]byte, error) {
	raw, _ := json.Marshal(body)
	resp, err := httpClient.Post(rawURL, "application/json", strings.NewReader(string(raw)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if dest != nil {
		if err := json.Unmarshal(b, dest); err != nil {
			return b, err
		}
	}
	return b, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func HostName(c *gin.Context) string {
	h := ctxutil.Host(c)
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	return h
}
