package wechat

import (
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"sort"
	"strings"
	"testing"
)

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
}

func TestParsePayNotify(t *testing.T) {
	xmlRaw := []byte(`<xml><out_trade_no>20240101120000123456</out_trade_no><transaction_id>wx123</transaction_id><attach>recharge</attach><result_code>SUCCESS</result_code></xml>`)
	n := ParsePayNotify(xmlRaw, nil)
	if !n.Paid || n.Attach != "recharge" || RechargeSN(n.OutTradeNo) != "202401011200001234" {
		t.Fatalf("xml notify %+v sn=%s", n, RechargeSN(n.OutTradeNo))
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
