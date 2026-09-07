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

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/tenantdb"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

func WechatPrepay(c *gin.Context, order model.RechargeOrder, paySN string, terminal int, from, redirect string) (any, error) {
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
		return nil, fmt.Errorf("请先完成微信渠道配置")
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
	path := "/v3/pay/transactions/native"
	switch terminal {
	case wechat.TerminalMNP, wechat.TerminalOA:
		path = "/v3/pay/transactions/jsapi"
		openid := lookupOpenid(order.TenantID, order.UserID, terminal)
		if openid == "" {
			return nil, fmt.Errorf("请先完成微信授权")
		}
		body["payer"] = map[string]any{"openid": openid}
	case wechat.TerminalH5:
		path = "/v3/pay/transactions/h5"
		body["scene_info"] = map[string]any{
			"payer_client_ip": ctxutil.ClientIP(c),
			"h5_info":         map[string]any{"type": "Wap"},
		}
	case 5, 6:
		path = "/v3/pay/transactions/app"
	}
	raw, _ := json.Marshal(body)
	result, err := wechatV3Post(cfg, key, path, raw)
	if err != nil {
		return nil, err
	}
	if msg := util.ToString(result["message"]); msg != "" && result["prepay_id"] == nil && result["code_url"] == nil && result["h5_url"] == nil {
		return nil, fmt.Errorf("微信:%s-%s", util.ToString(result["code"]), msg)
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
		ret := ctxutil.Domain(c) + "/mobile" + redirect + "?id=" + util.ToString(order.ID) + "&from=" + from + "&checkPay=true"
		return gin.H{"config": h5 + "&redirect_url=" + url.QueryEscape(ret), "pay_way": WayWechat}, nil
	case 5, 6:
		return gin.H{"config": util.ToString(result["prepay_id"]), "pay_way": WayWechat}, nil
	default:
		codeURL := util.ToString(result["code_url"])
		if codeURL == "" {
			return nil, fmt.Errorf("微信下单失败")
		}
		return gin.H{"config": codeURL, "pay_way": WayWechat}, nil
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
	if msg := util.ToString(result["message"]); msg != "" && util.ToString(result["status"]) == "" {
		return fmt.Errorf("微信退款:%s", msg)
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
	req, err := http.NewRequest(method, "https://api.mch.weixin.qq.com"+path, reader)
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
	case 5, 6:
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
