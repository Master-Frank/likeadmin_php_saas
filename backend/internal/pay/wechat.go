package pay

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"os"
	"strings"

	"likeadmin/backend/internal/config"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

var wechatAPIBase = "https://api.mch.weixin.qq.com"

func WechatPrepay(c *gin.Context, order model.RechargeOrder, paySN string, terminal int, from, redirect string) (any, error) {
	path, err := wechatPayPath(terminal)
	if err != nil {
		return nil, err
	}
	cfg := WechatCfg(c)
	if cfg.MchID == "" || cfg.APIClientKey == "" {
		return nil, fmt.Errorf("请先完成支付渠道配置")
	}
	key, err := parseRSAPrivateKey(cfg.APIClientKey)
	if err != nil {
		return nil, fmt.Errorf("微信支付私钥无效")
	}
	if cfg.SerialNo == "" {
		return nil, fmt.Errorf("微信支付证书无效")
	}
	appID := wechatAppID(c, terminal)
	if appID == "" {
		return nil, fmt.Errorf("%s", wechatChannelMissing(terminal))
	}
	notifyURL := ctxutil.Domain(c) + notifyPath(terminal)
	amount := int(order.OrderAmount*100 + 0.5)
	body := map[string]any{
		"appid":        appID,
		"mchid":        cfg.MchID,
		"description":  payDesc(from),
		"out_trade_no": paySN,
		"notify_url":   notifyURL,
		"amount":       map[string]any{"total": amount},
		"attach":       from,
	}
	switch terminal {
	case wechat.TerminalMNP, wechat.TerminalOA:
		openid := lookupOpenid(order.TenantID, order.UserID, terminal)
		if openid == "" {
			return nil, fmt.Errorf("请先完成微信授权")
		}
		body["payer"] = map[string]any{"openid": openid}
	case wechat.TerminalH5:
		body["scene_info"] = map[string]any{
			"payer_client_ip": debugPayOverride("LIKEADMIN_TEST_WEB_IP", ctxutil.ClientIP(c)),
			"h5_info":         map[string]any{"type": "Wap"},
		}
	}
	raw, _ := json.Marshal(body)
	result, err := wechatV3Post(cfg, key, path, raw)
	if err != nil {
		return nil, err
	}
	if err := wechatResultFail(result); err != nil {
		return nil, err
	}
	switch terminal {
	case wechat.TerminalMNP, wechat.TerminalOA:
		prepayID := util.ToString(result["prepay_id"])
		if prepayID == "" {
			return nil, fmt.Errorf("微信下单失败")
		}
		bridge, err := jsapiBridge(key, appID, prepayID)
		if err != nil {
			return nil, err
		}
		return gin.H{"config": bridge, "pay_way": WayWechat}, nil
	case wechat.TerminalH5:
		h5 := util.ToString(result["h5_url"])
		if h5 == "" {
			return nil, fmt.Errorf("微信下单失败")
		}
		ret := debugPayOverride("LIKEADMIN_TEST_WEB_DOMAIN", ctxutil.Domain(c)) + "/mobile" + redirect + "?id=" + util.ToString(order.ID) + "&from=" + from + "&checkPay=true"
		return gin.H{"config": h5 + "&redirect_url=" + url.QueryEscape(ret), "pay_way": WayWechat}, nil
	case wechat.TerminalIOS, wechat.TerminalAndroid:
		return gin.H{"config": util.ToString(result["prepay_id"]), "pay_way": WayWechat}, nil
	default:
		codeURL := util.ToString(result["code_url"])
		if codeURL == "" {
			return nil, fmt.Errorf("微信下单失败")
		}
		return gin.H{"config": codeURL, "pay_way": WayWechat}, nil
	}
}

// debugPayOverride mirrors PHP WeChatPayService mwebPay test_web_ip/domain when APP_DEBUG.
func debugPayOverride(envKey, fallback string) string {
	if !config.C.App.Debug {
		return fallback
	}
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		return v
	}
	return fallback
}

// wechatPayPath mirrors PHP WeChatPayService::pay terminal switch.
func wechatPayPath(terminal int) (string, error) {
	switch terminal {
	case wechat.TerminalMNP, wechat.TerminalOA:
		return "/v3/pay/transactions/jsapi", nil
	case wechat.TerminalH5:
		return "/v3/pay/transactions/h5", nil
	case wechat.TerminalIOS, wechat.TerminalAndroid:
		return "/v3/pay/transactions/app", nil
	case wechat.TerminalPC:
		return "/v3/pay/transactions/native", nil
	default:
		return "", fmt.Errorf("支付方式错误")
	}
}

func WechatRefund(c *gin.Context, transactionID, refundSN string, refundAmount, totalAmount float64) error {
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	return WechatRefundByTenant(tid, transactionID, refundSN, refundAmount, totalAmount)
}

func WechatRefundByTenant(tenantID uint, transactionID, refundSN string, refundAmount, totalAmount float64) error {
	cfg := WechatCfgByTenant(tenantID)
	if cfg.MchID == "" || cfg.APIClientKey == "" {
		return fmt.Errorf("请先完成支付渠道配置")
	}
	if transactionID == "" {
		return fmt.Errorf("第三方交易号缺失")
	}
	key, err := parseRSAPrivateKey(cfg.APIClientKey)
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{
		"transaction_id": transactionID,
		"out_refund_no":  refundSN,
		"amount": map[string]any{
			"refund": int(refundAmount*100 + 0.5), "total": int(totalAmount*100 + 0.5), "currency": "CNY",
		},
	})
	result, err := wechatV3Post(cfg, key, "/v3/refund/domestic/refunds", body)
	if err != nil {
		return err
	}
	return wechatResultFail(result)
}

// wechatResultFail mirrors PHP WeChatPayService::checkResultFail.
func wechatResultFail(result map[string]any) error {
	if result == nil {
		return nil
	}
	code := util.ToString(result["code"])
	message := util.ToString(result["message"])
	if code != "" || message != "" {
		return fmt.Errorf("微信:%s-%s", code, message)
	}
	return nil
}

func WechatQueryRefund(cfg WechatPayCfg, refundSN string) (map[string]any, error) {
	if cfg.MchID == "" || cfg.APIClientKey == "" || refundSN == "" {
		return nil, nil
	}
	key, err := parseRSAPrivateKey(cfg.APIClientKey)
	if err != nil {
		return nil, err
	}
	return wechatV3Get(cfg, key, "/v3/refund/domestic/refunds/"+url.PathEscape(refundSN))
}

func ParseWechatRefundQuery(result map[string]any) (ok bool, msg string, known bool) {
	if result == nil {
		return false, "", false
	}
	if util.ToString(result["status"]) == "SUCCESS" {
		return true, "", true
	}
	code := util.ToString(result["code"])
	message := util.ToString(result["message"])
	if code != "" || message != "" {
		return false, code + "-" + message, true
	}
	return false, "", false
}

// RefundQueryTradeNo picks the gateway refund/trade id from a WeChat or Ali query body.
func RefundQueryTradeNo(result map[string]any) string {
	if result == nil {
		return ""
	}
	for _, k := range []string{"refund_id", "trade_no", "tradeNo", "transaction_id"} {
		if v := util.ToString(result[k]); v != "" {
			return v
		}
	}
	return ""
}

func wechatV3Get(cfg WechatPayCfg, key *rsa.PrivateKey, path string) (map[string]any, error) {
	return wechatV3Do(cfg, key, http.MethodGet, path, nil)
}

func wechatV3Post(cfg WechatPayCfg, key *rsa.PrivateKey, path string, body []byte) (map[string]any, error) {
	return wechatV3Do(cfg, key, http.MethodPost, path, body)
}

func wechatV3Do(cfg WechatPayCfg, key *rsa.PrivateKey, method, path string, body []byte) (map[string]any, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randomHex(16)
	bodyStr := ""
	if len(body) > 0 {
		bodyStr = string(body)
	}
	msg := method + "\n" + path + "\n" + ts + "\n" + nonce + "\n" + bodyStr + "\n"
	sig, err := rsaSHA256Base64(key, msg)
	if err != nil {
		return nil, err
	}
	auth := fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`,
		cfg.MchID, nonce, ts, cfg.SerialNo, sig)
	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, wechatAPIBase+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return nil, fmt.Errorf("微信响应无效")
	}
	return out, nil
}

func jsapiBridge(key *rsa.PrivateKey, appID, prepayID string) (map[string]any, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randomHex(16)
	pkg := "prepay_id=" + prepayID
	msg := appID + "\n" + ts + "\n" + nonce + "\n" + pkg + "\n"
	sig, err := rsaSHA256Base64(key, msg)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"appId": appID, "timeStamp": ts, "nonceStr": nonce,
		"package": pkg, "signType": "RSA", "paySign": sig,
	}, nil
}

func wechatChannelMissing(terminal int) string {
	switch terminal {
	case wechat.TerminalMNP:
		return "请先设置小程序配置"
	case wechat.TerminalOA, wechat.TerminalH5, wechat.TerminalPC:
		return "请先设置公众号配置"
	default:
		return "请先设置小程序配置"
	}
}

func wechatAppID(c *gin.Context, terminal int) string {
	switch terminal {
	case wechat.TerminalMNP:
		id, _ := wechat.MnpConfig(c)
		return id
	case wechat.TerminalPC, wechat.TerminalH5, wechat.TerminalOA:
		id, _, _ := wechat.OAConfig(c)
		return id
	default:
		id, _ := wechat.OpenConfig(c)
		if id == "" {
			id, _, _ = wechat.OAConfig(c)
		}
		return id
	}
}

func notifyPath(terminal int) string {
	switch terminal {
	case wechat.TerminalMNP:
		return "/api/pay/notifyMnp"
	case wechat.TerminalIOS, wechat.TerminalAndroid:
		return "/api/pay/notifyApp"
	default:
		return "/api/pay/notifyOa"
	}
}

func payDesc(from string) string {
	if from == "recharge" {
		return "充值"
	}
	return "商品"
}

func lookupOpenid(tenantID, userID uint, terminal int) string {
	db := tenantdb.ForTenant(tenantID)
	if db == nil {
		return ""
	}
	q := db.Where("user_id = ? AND terminal = ?", userID, terminal)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	var auth model.UserAuth
	if q.First(&auth).Error == nil {
		return auth.Openid
	}
	return ""
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
