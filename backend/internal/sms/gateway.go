package sms

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/cfgsvc"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

var gatewayClient = &http.Client{Timeout: 8 * time.Second}

type engineCfg struct {
	Name      string
	Status    int
	Sign      string
	AppKey    string
	SecretKey string
	SecretID  string
	AppID     string
}

func maybeGatewaySend(c *gin.Context, mobile string, scene int, code string, logID uint) error {
	if c == nil || bootstrap.DB == nil {
		return nil
	}
	engine := strings.ToUpper(strings.TrimSpace(cfgsvc.GetString(c, "sms", "engine", "")))
	if engine == "" {
		return nil
	}
	cfg := loadEngine(c, engine)
	if cfg.Status != 1 {
		return nil
	}
	notice := loadNoticeSMS(c, scene)
	tplID := util.ToString(notice["template_id"])
	if tplID == "" {
		return nil
	}
	content := formatContent(util.ToString(notice["content"]), map[string]string{"code": code, "mobile": mobile})
	var (
		result any
		err    error
	)
	switch engine {
	case "ALI":
		if cfg.AppKey == "" || cfg.SecretKey == "" || cfg.Sign == "" {
			return nil
		}
		result, err = sendAliyun(cfg, mobile, tplID, code)
	case "TENCENT":
		if cfg.SecretID == "" || cfg.SecretKey == "" || cfg.Sign == "" || cfg.AppID == "" {
			return nil
		}
		result, err = sendTencent(cfg, mobile, tplID, tencentParams(notice, code, mobile))
	default:
		return nil
	}
	if logID > 0 {
		raw, _ := json.Marshal(result)
		if err != nil {
			raw, _ = json.Marshal(err.Error())
			bootstrap.DB.Model(&model.TenantSmsLog{}).Where("id = ?", logID).Updates(map[string]any{
				"send_status": 2, "results": string(raw), "content": content,
			})
		} else {
			bootstrap.DB.Model(&model.TenantSmsLog{}).Where("id = ?", logID).Updates(map[string]any{
				"send_status": 1, "results": string(raw), "content": content,
			})
		}
	}
	return err
}

func loadEngine(c *gin.Context, engine string) engineCfg {
	name := strings.ToLower(engine)
	if name == "aliyun" {
		name = "ali"
	}
	raw := cfgsvc.Get(c, "sms", name, map[string]any{})
	m, _ := raw.(map[string]any)
	if m == nil {
		return engineCfg{}
	}
	return engineCfg{
		Name:      util.ToString(m["name"]),
		Status:    util.ToInt(m["status"]),
		Sign:      util.ToString(m["sign"]),
		AppKey:    firstNonEmpty(util.ToString(m["app_key"]), util.ToString(m["access_key"])),
		SecretKey: util.ToString(m["secret_key"]),
		SecretID:  util.ToString(m["secret_id"]),
		AppID:     util.ToString(m["app_id"]),
	}
}

func loadNoticeSMS(c *gin.Context, scene int) map[string]any {
	var raw string
	q := bootstrap.DB.Model(&model.TenantNoticeSetting{}).Where("scene_id = ?", scene).Select("sms_notice")
	if tid := ctxutil.Get(c).TenantID; tid > 0 {
		q = q.Where("tenant_id = ?", tid)
	}
	if q.Scan(&raw).Error != nil || raw == "" {
		bootstrap.DB.Model(&model.NoticeSetting{}).Where("scene_id = ?", scene).Select("sms_notice").Scan(&raw)
	}
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) != nil {
		return map[string]any{}
	}
	return m
}

func formatContent(tpl string, vars map[string]string) string {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "${"+k+"}", v)
	}
	if out == "" {
		return "验证码" + vars["code"]
	}
	return out
}

func tencentParams(notice map[string]any, code, mobile string) []string {
	return tencentParamsFrom(util.ToString(notice["content"]), map[string]string{
		"code": code, "mobile": mobile,
	})
}

// tencentParamsFrom matches PHP SmsMessageService::setSmsParams for TENCENT:
// collect ${key} placeholders, then order by strpos($content, $key).
func tencentParamsFrom(content string, params map[string]string) []string {
	type item struct {
		pos int
		key string
	}
	found := make([]item, 0, len(params))
	seen := map[string]bool{}
	for k := range params {
		if k == "" || seen[k] || !strings.Contains(content, "${"+k+"}") {
			continue
		}
		seen[k] = true
		pos := strings.Index(content, k)
		if pos < 0 {
			pos = 0
		}
		found = append(found, item{pos: pos, key: k})
	}
	if len(found) == 0 {
		if code := params["code"]; code != "" {
			return []string{code}
		}
		return nil
	}
	for i := 0; i < len(found); i++ {
		for j := i + 1; j < len(found); j++ {
			if found[j].pos < found[i].pos {
				found[i], found[j] = found[j], found[i]
			}
		}
	}
	out := make([]string, 0, len(found))
	for _, it := range found {
		out = append(out, params[it.key])
	}
	return out
}

func sendAliyun(cfg engineCfg, mobile, templateID, code string) (any, error) {
	tpl, _ := json.Marshal(map[string]string{"code": code})
	params := map[string]string{
		"AccessKeyId":      cfg.AppKey,
		"Action":           "SendSms",
		"Format":           "JSON",
		"PhoneNumbers":     mobile,
		"RegionId":         "cn-hangzhou",
		"SignName":         cfg.Sign,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureNonce":   strconv.FormatInt(time.Now().UnixNano(), 10),
		"SignatureVersion": "1.0",
		"TemplateCode":     templateID,
		"TemplateParam":    string(tpl),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Version":          "2017-05-25",
	}
	params["Signature"] = aliRPCSign(cfg.SecretKey, params)
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := gatewayClient.PostForm("https://dysmsapi.aliyuncs.com/", form)
	if err != nil {
		return nil, fmt.Errorf("阿里云短信错误：%s", err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if json.Unmarshal(body, &out) != nil {
		return nil, fmt.Errorf("阿里云短信错误：响应无效")
	}
	if util.ToString(out["Code"]) != "OK" {
		return out, fmt.Errorf("阿里云短信错误：%s", firstNonEmpty(util.ToString(out["Message"]), string(body)))
	}
	return out, nil
}

func sendTencent(cfg engineCfg, mobile, templateID string, tplParams []string) (any, error) {
	payloadMap := map[string]any{
		"PhoneNumberSet":   []string{"+86" + mobile},
		"TemplateID":       templateID,
		"Sign":             cfg.Sign,
		"TemplateParamSet": tplParams,
		"SmsSdkAppid":      cfg.AppID,
	}
	payload, _ := json.Marshal(payloadMap)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	req, err := http.NewRequest(http.MethodPost, "https://sms.tencentcloudapi.com/", strings.NewReader(string(payload)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", "sms.tencentcloudapi.com")
	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", "2019-07-11")
	req.Header.Set("X-TC-Timestamp", ts)
	req.Header.Set("X-TC-Region", "ap-guangzhou")
	req.Header.Set("Authorization", tencentAuth(cfg.SecretID, cfg.SecretKey, string(payload), ts))
	resp, err := gatewayClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("腾讯云短信错误：%s", err.Error())
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	if json.Unmarshal(body, &out) != nil {
		return nil, fmt.Errorf("腾讯云短信错误：响应无效")
	}
	if respObj, ok := out["Response"].(map[string]any); ok {
		if e, ok := respObj["Error"].(map[string]any); ok {
			return out, fmt.Errorf("腾讯云短信错误：%s", util.ToString(e["Message"]))
		}
		if set, ok := respObj["SendStatusSet"].([]any); ok && len(set) > 0 {
			if first, ok := set[0].(map[string]any); ok && !strings.EqualFold(util.ToString(first["Code"]), "Ok") {
				return out, fmt.Errorf("腾讯云短信错误：%s", firstNonEmpty(util.ToString(first["Message"]), util.ToString(first["Code"])))
			}
		}
	}
	return out, nil
}

func aliRPCSign(secret string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('&')
		}
		b.WriteString(aliPercentEncode(k))
		b.WriteByte('=')
		b.WriteString(aliPercentEncode(params[k]))
	}
	stringToSign := "POST&" + aliPercentEncode("/") + "&" + aliPercentEncode(b.String())
	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func aliPercentEncode(s string) string {
	enc := url.QueryEscape(s)
	enc = strings.ReplaceAll(enc, "+", "%20")
	enc = strings.ReplaceAll(enc, "*", "%2A")
	enc = strings.ReplaceAll(enc, "%7E", "~")
	return enc
}

func tencentAuth(secretID, secretKey, payload, timestamp string) string {
	hashed := sha256.Sum256([]byte(payload))
	canonical := "POST\n/\n\ncontent-type:application/json\nhost:sms.tencentcloudapi.com\n\ncontent-type;host\n" + hex.EncodeToString(hashed[:])
	canHash := sha256.Sum256([]byte(canonical))
	date := time.Unix(mustInt64(timestamp), 0).UTC().Format("2006-01-02")
	scope := date + "/sms/tc3_request"
	stringToSign := "TC3-HMAC-SHA256\n" + timestamp + "\n" + scope + "\n" + hex.EncodeToString(canHash[:])
	secretDate := hmacSHA256([]byte("TC3"+secretKey), date)
	secretService := hmacSHA256(secretDate, "sms")
	secretSigning := hmacSHA256(secretService, "tc3_request")
	sig := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s", secretID, scope, sig)
}

func hmacSHA256(key []byte, msg string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(msg))
	return mac.Sum(nil)
}

func mustInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
