package pay

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

var aliGatewayURL = "https://openapi.alipay.com/gateway.do"

// aliPayWay mirrors PHP AliPayService::pay terminal match.
func aliPayWay(terminal int) (method, productCode string, err error) {
	switch terminal {
	case wechat.TerminalPC:
		return "alipay.trade.page.pay", "FAST_INSTANT_TRADE_PAY", nil
	case wechat.TerminalIOS, wechat.TerminalAndroid:
		return "alipay.trade.app.pay", "QUICK_MSECURITY_PAY", nil
	case wechat.TerminalOA, wechat.TerminalH5:
		return "alipay.trade.wap.pay", "QUICK_WAP_WAY", nil
	default:
		return "", "", fmt.Errorf("支付方式错误")
	}
}

func AliPrepay(c *gin.Context, order model.RechargeOrder, from, redirect string, terminal int) (any, error) {
	// PHP AliPayService::pay match has no MNP case; default is 支付方式错误
	// even when app_id is empty.
	if terminal == wechat.TerminalMNP {
		return nil, fmt.Errorf("支付方式错误")
	}
	cfg := AliCfg(c)
	if cfg.AppID == "" || cfg.PrivateKey == "" {
		return nil, fmt.Errorf("请配置好支付设置")
	}
	key, err := parseRSAPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("支付宝私钥无效")
	}
	method, productCode, err := aliPayWay(terminal)
	if err != nil {
		return nil, err
	}
	notifyURL := ctxutil.Domain(c) + "/api/pay/aliNotify"
	domain := ctxutil.Domain(c)
	returnURL := domain + redirect
	if terminal == wechat.TerminalOA || terminal == wechat.TerminalH5 {
		returnURL = domain + "/mobile" + redirect + "?id=" + util.ToString(order.ID) + "&from=" + from + "&checkPay=true"
	}
	subject := "订单:" + order.SN
	if terminal == wechat.TerminalIOS || terminal == wechat.TerminalAndroid {
		subject = order.SN
	}
	bizJSON, _ := json.Marshal(map[string]any{
		"out_trade_no":    order.SN,
		"total_amount":    fmt.Sprintf("%.2f", order.OrderAmount),
		"subject":         subject,
		"product_code":    productCode,
		"passback_params": from,
	})
	params := map[string]string{
		"app_id":      cfg.AppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  notifyURL,
		"biz_content": string(bizJSON),
	}
	if method != "alipay.trade.app.pay" {
		params["return_url"] = returnURL
	}
	attachAliCertSNs(params, cfg)
	sig, err := rsaSHA256Base64(key, aliSignContent(params))
	if err != nil {
		return nil, err
	}
	params["sign"] = sig
	if method == "alipay.trade.app.pay" {
		return gin.H{"config": aliQuery(params), "pay_way": WayAli}, nil
	}
	return gin.H{"config": aliForm(params), "pay_way": WayAli}, nil
}

func AliVerifyNotify(c *gin.Context, form map[string][]string) bool {
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	return AliVerifyNotifyByTenant(tid, form)
}

func AliVerifyNotifyByTenant(tenantID uint, form map[string][]string) bool {
	return aliVerifyForm(AliCfgByTenant(tenantID), form)
}

func aliVerifyForm(cfg AliPayCfg, form map[string][]string) bool {
	pub := resolveAliPublicKey(cfg)
	if pub == nil {
		// PHP AliPayService::notify always verifyNotify(); missing public key fails.
		return false
	}
	sign := firstForm(form, "sign")
	if sign == "" {
		return false
	}
	params := map[string]string{}
	for k, vs := range form {
		if k == "sign" || k == "sign_type" || len(vs) == 0 {
			continue
		}
		params[k] = vs[0]
	}
	return verifyRSA2(pub, aliSignContent(params), sign)
}

func resolveAliPublicKey(cfg AliPayCfg) *rsa.PublicKey {
	if cfg.AliPublicKey != "" {
		if pub, err := parseRSAPublicKey(cfg.AliPublicKey); err == nil {
			return pub
		}
	}
	if cfg.Mode == "certificate" || cfg.AliPublicCert != "" {
		if cert, err := parseCertificate(cfg.AliPublicCert); err == nil {
			if pub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
				return pub
			}
		}
	}
	return nil
}

// SignAliNotify RSA2-signs the Alipay notify content (same string as verify).
func SignAliNotify(key *rsa.PrivateKey, params map[string]string) (string, error) {
	if key == nil {
		return "", fmt.Errorf("missing key")
	}
	return rsaSHA256Base64(key, aliSignContent(params))
}

func aliSignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

func aliQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	vals := url.Values{}
	for _, k := range keys {
		vals.Set(k, params[k])
	}
	return vals.Encode()
}

func aliForm(params map[string]string) string {
	var b strings.Builder
	b.WriteString(`<form id="alipaysubmit" name="alipaysubmit" action="`)
	b.WriteString(html.EscapeString(aliGatewayURL))
	b.WriteString(`?charset=utf-8" method="POST">`)
	for k, v := range params {
		b.WriteString(`<input type="hidden" name="`)
		b.WriteString(html.EscapeString(k))
		b.WriteString(`" value="`)
		b.WriteString(html.EscapeString(v))
		b.WriteString(`"/>`)
	}
	b.WriteString(`<input type="submit" value="ok" style="display:none;"></form><script>document.forms['alipaysubmit'].submit();</script>`)
	return b.String()
}

func firstForm(form map[string][]string, key string) string {
	if vs := form[key]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

type AliRefundResult struct {
	OK         bool
	Code       string
	Msg        string
	FundChange string
	TradeNo    string
	Raw        map[string]any
}

func AliRefund(c *gin.Context, orderSN, refundSN string, amount float64) (AliRefundResult, error) {
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	return AliRefundByTenant(tid, orderSN, refundSN, amount)
}

func AliRefundByTenant(tenantID uint, orderSN, refundSN string, amount float64) (AliRefundResult, error) {
	cfg := AliCfgByTenant(tenantID)
	if cfg.AppID == "" || cfg.PrivateKey == "" {
		return AliRefundResult{}, fmt.Errorf("请先完成支付渠道配置")
	}
	if orderSN == "" {
		return AliRefundResult{}, fmt.Errorf("订单号缺失")
	}
	key, err := parseRSAPrivateKey(cfg.PrivateKey)
	if err != nil {
		return AliRefundResult{}, err
	}
	biz, _ := json.Marshal(map[string]any{
		"out_trade_no":   orderSN,
		"refund_amount":  fmt.Sprintf("%.2f", amount),
		"out_request_no": refundSN,
	})
	params := map[string]string{
		"app_id":      cfg.AppID,
		"method":      "alipay.trade.refund",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(biz),
	}
	attachAliCertSNs(params, cfg)
	sig, err := rsaSHA256Base64(key, aliSignContent(params))
	if err != nil {
		return AliRefundResult{}, err
	}
	params["sign"] = sig
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).PostForm(aliGatewayURL, form)
	if err != nil {
		return AliRefundResult{}, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return AliRefundResult{}, fmt.Errorf("支付宝退款响应无效")
	}
	body, _ := out["alipay_trade_refund_response"].(map[string]any)
	res := ParseAliRefundBody(body)
	if res.Code != "" && res.Code != "10000" {
		return res, fmt.Errorf("支付宝退款:%s", firstNonEmpty(util.ToString(body["sub_msg"]), res.Msg))
	}
	return res, nil
}

func AliQueryRefundByTenant(tenantID uint, orderSN, refundSN string) (map[string]any, error) {
	cfg := AliCfgByTenant(tenantID)
	if cfg.AppID == "" || cfg.PrivateKey == "" || orderSN == "" || refundSN == "" {
		return nil, nil
	}
	key, err := parseRSAPrivateKey(cfg.PrivateKey)
	if err != nil {
		return nil, err
	}
	biz, _ := json.Marshal(map[string]any{
		"out_trade_no":   orderSN,
		"out_request_no": refundSN,
	})
	params := map[string]string{
		"app_id":      cfg.AppID,
		"method":      "alipay.trade.fastpay.refund.query",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(biz),
	}
	attachAliCertSNs(params, cfg)
	sig, err := rsaSHA256Base64(key, aliSignContent(params))
	if err != nil {
		return nil, err
	}
	params["sign"] = sig
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := (&http.Client{Timeout: 8 * time.Second}).PostForm(aliGatewayURL, form)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if json.Unmarshal(raw, &out) != nil {
		return nil, fmt.Errorf("支付宝退款查询响应无效")
	}
	body, _ := out["alipay_trade_fastpay_refund_query_response"].(map[string]any)
	return body, nil
}

// ParseAliRefundQuery maps alipay.trade.fastpay.refund.query to the same
// ok/msg/known triple as ParseWechatRefundQuery.
func ParseAliRefundQuery(body map[string]any) (ok bool, msg string, known bool) {
	if body == nil {
		return false, "", false
	}
	status := firstNonEmpty(util.ToString(body["refund_status"]), util.ToString(body["refundStatus"]))
	if status == "REFUND_SUCCESS" {
		return true, "", true
	}
	code := util.ToString(body["code"])
	if code != "" && code != "10000" {
		return false, firstNonEmpty(util.ToString(body["sub_msg"]), util.ToString(body["msg"])), true
	}
	if status != "" {
		return false, status, true
	}
	return false, "", false
}

// ParseAliRefundBody mirrors PHP RefundLogic::aliPayRefund success checks.
func ParseAliRefundBody(body map[string]any) AliRefundResult {
	res := AliRefundResult{Raw: body}
	if body == nil {
		return res
	}
	res.Code = util.ToString(body["code"])
	res.Msg = util.ToString(body["msg"])
	res.FundChange = util.ToString(body["fund_change"])
	if res.FundChange == "" {
		res.FundChange = util.ToString(body["fundChange"])
	}
	res.TradeNo = util.ToString(body["trade_no"])
	if res.TradeNo == "" {
		res.TradeNo = util.ToString(body["tradeNo"])
	}
	res.OK = res.Code == "10000" && res.Msg == "Success" && res.FundChange == "Y"
	return res
}

func aliSignWithKey(key *rsa.PrivateKey, params map[string]string) (string, error) {
	return rsaSHA256Base64(key, aliSignContent(params))
}
