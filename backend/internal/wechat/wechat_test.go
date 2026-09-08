package wechat

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestWechatCacheTTL(t *testing.T) {
	if _, ok := wechatCacheTTL(0); ok {
		t.Fatal("expires_in=0 should not cache")
	}
	if _, ok := wechatCacheTTL(200); ok {
		t.Fatal("expires_in<=200 should not cache")
	}
	ttl, ok := wechatCacheTTL(7200)
	if !ok || ttl != 7000*time.Second {
		t.Fatalf("ttl=%v ok=%v", ttl, ok)
	}
}

func TestCheckOASignature(t *testing.T) {
	if !CheckOASignature("", "", "", "") {
		t.Fatal("empty token should pass")
	}
	if CheckOASignature("token", "bad", "1", "2") {
		t.Fatal("bad signature accepted")
	}
	arr := []string{"likeadmin", "1710000000", "nonce"}
	sort.Strings(arr)
	sum := sha1.Sum([]byte(strings.Join(arr, "")))
	sig := hex.EncodeToString(sum[:])
	if !CheckOASignature("likeadmin", sig, "1710000000", "nonce") {
		t.Fatalf("expected valid signature %s", sig)
	}
}

func TestMatchReply(t *testing.T) {
	rows := []ReplyRow{
		{ReplyType: ReplyFollow, Status: 1, Content: "欢迎"},
		{ReplyType: ReplyKeyword, MatchingType: MatchFull, Keyword: "你好", Content: "全匹配", Status: 1, Sort: 1},
		{ReplyType: ReplyKeyword, MatchingType: MatchFuzzy, Keyword: "帮助", Content: "模糊", Status: 1, Sort: 2},
		{ReplyType: ReplyDefault, Status: 1, Content: "默认"},
	}
	if got := MatchReply(OAMessage{MsgType: "event", Event: "subscribe"}, rows); got != "欢迎" {
		t.Fatalf("follow=%s", got)
	}
	if got := MatchReply(OAMessage{MsgType: "text", Content: "你好"}, rows); got != "全匹配" {
		t.Fatalf("full=%s", got)
	}
	if got := MatchReply(OAMessage{MsgType: "text", Content: "需要帮助吗"}, rows); got != "模糊" {
		t.Fatalf("fuzzy=%s", got)
	}
	if got := MatchReply(OAMessage{MsgType: "text", Content: "其他"}, rows); got != "默认" {
		t.Fatalf("default=%s", got)
	}
	tied := []ReplyRow{
		{ReplyType: ReplyKeyword, MatchingType: MatchFull, Keyword: "hi", Content: "先", Status: 1, Sort: 1},
		{ReplyType: ReplyKeyword, MatchingType: MatchFull, Keyword: "hi", Content: "后", Status: 1, Sort: 1},
	}
	if got := MatchReply(OAMessage{MsgType: "text", Content: "hi"}, tied); got != "先" {
		t.Fatalf("first match=%s", got)
	}
	reversed := []ReplyRow{
		{ID: 2, ReplyType: ReplyKeyword, MatchingType: MatchFull, Keyword: "hi", Content: "后", Status: 1, Sort: 2},
		{ID: 1, ReplyType: ReplyKeyword, MatchingType: MatchFull, Keyword: "hi", Content: "先", Status: 1, Sort: 1},
	}
	if got := MatchReply(OAMessage{MsgType: "text", Content: "hi"}, reversed); got != "先" {
		t.Fatalf("sort asc=%s", got)
	}
	follows := []ReplyRow{
		{ID: 2, ReplyType: ReplyFollow, Status: 1, Content: "新", Sort: 1},
		{ID: 1, ReplyType: ReplyFollow, Status: 1, Content: "旧", Sort: 9},
		{ID: 3, ReplyType: ReplyDefault, Status: 1, Content: "默认"},
	}
	if got := MatchReply(OAMessage{MsgType: "event", Event: "subscribe"}, follows); got != "旧" {
		t.Fatalf("follow value()=%s", got)
	}
}

func TestCodeURLEmptyRedirect(t *testing.T) {
	u := CodeURL("wxapp", "")
	if !strings.Contains(u, "appid=wxapp") || !strings.Contains(u, "redirect_uri=") {
		t.Fatalf("%s", u)
	}
	scan := ScanCodeURL("wxapp", "", "st")
	if !strings.Contains(scan, "state=st") || !strings.Contains(scan, "redirect_uri=") {
		t.Fatalf("%s", scan)
	}
}

func TestJSSDKConfig(t *testing.T) {
	cfg := jsSDKConfig("wxapp", 1, "n", "sig")
	if cfg["appId"] != "wxapp" || cfg["debug"] != false {
		t.Fatalf("%v", cfg)
	}
	list, _ := cfg["jsApiList"].([]string)
	if len(list) != 12 || list[0] != "onMenuShareTimeline" || list[len(list)-1] != "scanQRCode" {
		t.Fatalf("jsApiList %v", list)
	}
	open, _ := cfg["openTagList"].([]string)
	if open == nil || len(open) != 0 {
		t.Fatalf("openTagList %v", open)
	}
}

func TestWechatV2Sign(t *testing.T) {
	fields := map[string]string{
		"out_trade_no":   "SN1",
		"transaction_id": "wx1",
		"attach":         "recharge",
		"result_code":    "SUCCESS",
		"sign":           "OLD",
		"empty":          "",
	}
	sig := WechatV2Sign(fields, "apikey")
	if sig == "" || sig == "OLD" {
		t.Fatal(sig)
	}
	xmlRaw := []byte(`<xml><out_trade_no>SN1</out_trade_no><transaction_id>wx1</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code><sign>` + sig + `</sign></xml>`)
	if !VerifyWechatV2XML(xmlRaw, "apikey") {
		t.Fatal("valid sign rejected")
	}
	if VerifyWechatV2XML(xmlRaw, "wrong") {
		t.Fatal("bad key accepted")
	}
	unsigned := []byte(`<xml><out_trade_no>SN1</out_trade_no><result_code>SUCCESS</result_code></xml>`)
	if !VerifyWechatV2XML(unsigned, "apikey") {
		t.Fatal("unsigned xml should pass")
	}
}

func TestParsePayNotify(t *testing.T) {
	xmlRaw := []byte(`<xml><out_trade_no>20240101120000123456</out_trade_no><transaction_id>wx123</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code></xml>`)
	n := ParsePayNotify(xmlRaw, nil)
	if !n.Paid || n.Attach != "recharge" || RechargeSN(n.OutTradeNo) != "202401011200001234" {
		t.Fatalf("xml notify %+v sn=%s", n, RechargeSN(n.OutTradeNo))
	}
	loose := ParsePayNotify([]byte(`<xml><out_trade_no>SNFAIL</out_trade_no><return_code>SUCCESS</return_code><result_code>FAIL</result_code></xml>`), nil)
	if loose.Paid {
		t.Fatalf("return_code SUCCESS without result/trade SUCCESS must not be paid: %+v", loose)
	}
	ali := ParsePayNotify(nil, map[string][]string{
		"out_trade_no":    {"SN001"},
		"trade_no":        {"ALI1"},
		"passback_params": {"recharge"},
		"trade_status":    {"TRADE_SUCCESS"},
	})
	if !ali.Paid || ali.Attach != "recharge" || ali.OutTradeNo != "SN001" {
		t.Fatalf("ali %+v", ali)
	}
	empty := ParsePayNotify(xmlRaw, nil)
	empty.Attach = ""
	if ShouldMarkRechargePaid(empty) {
		t.Fatal("empty attach should not mark recharge paid")
	}
	if !ShouldMarkRechargePaid(n) {
		t.Fatal("recharge attach should mark paid")
	}
	rf := ParsePayNotify([]byte(`{"event_type":"REFUND.SUCCESS","out_refund_no":"RF1","refund_status":"SUCCESS"}`), nil)
	if !ShouldApplyRefund(rf) || rf.OutRefundNo != "RF1" {
		t.Fatalf("refund %+v", rf)
	}
	if ShouldApplyRefund(n) {
		t.Fatal("pay notify should not apply refund")
	}
}

func TestEncryptedReplyXMLUsesNowTimestamp(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 3)
	}
	aesKey := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	xmlBody := TextReplyXML("user", "oa", "hi")
	out, err := EncryptedReplyXML("token", aesKey, "wxappid", "", "nonce", xmlBody)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "<TimeStamp>0</TimeStamp>") {
		t.Fatal("empty timestamp must not stay 0")
	}
	if !strings.Contains(out, "<TimeStamp>") || !strings.Contains(out, "<Nonce><![CDATA[nonce]]></Nonce>") {
		t.Fatalf("envelope %s", out)
	}
}

func TestTextReplyXML(t *testing.T) {
	s := TextReplyXML("user", "oa", "hi")
	if !containsAll(s, "user", "oa", "hi", "text") {
		t.Fatalf("xml %s", s)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func TestOACryptoRoundtrip(t *testing.T) {
	rawKey := make([]byte, 32)
	for i := range rawKey {
		rawKey[i] = byte(i + 3)
	}
	aesKey := strings.TrimRight(base64.StdEncoding.EncodeToString(rawKey), "=")
	xmlBody := `<xml><ToUserName><![CDATA[oa]]></ToUserName><FromUserName><![CDATA[user]]></FromUserName><CreateTime>1</CreateTime><MsgType><![CDATA[text]]></MsgType><Content><![CDATA[你好]]></Content></xml>`
	enc, err := EncryptOA(aesKey, "wxappid", xmlBody)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptOA(aesKey, enc)
	if err != nil {
		t.Fatal(err)
	}
	if plain.appID != "wxappid" {
		t.Fatalf("appid %s", plain.appID)
	}
	msg, err := ParseOAXML(plain.xml)
	if err != nil || msg.Content != "你好" {
		t.Fatalf("msg %+v err=%v", msg, err)
	}
	wrapped := "<xml><Encrypt><![CDATA[" + enc + "]]></Encrypt></xml>"
	got, err := DecodeOABody([]byte(wrapped), "token", aesKey, "wxappid", 3, "", "1", "n")
	if err != nil || got.Content != "你好" {
		t.Fatalf("decode %+v err=%v", got, err)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
