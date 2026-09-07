package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"likeadmin/backend/internal/wechat"
)

func TestVerifyWechatV3Signature(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"event_type":"TRANSACTION.SUCCESS"}`)
	ts, nonce := "1710000000", "nonce1"
	msg := ts + "\n" + nonce + "\n" + string(body) + "\n"
	sig, err := rsaSHA256Base64(key, msg)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyWechatV3Signature(ts, nonce, sig, body, []*rsa.PublicKey{&key.PublicKey}) {
		t.Fatal("valid v3 sign rejected")
	}
	if VerifyWechatV3Signature(ts, nonce, sig, []byte(`{}`), []*rsa.PublicKey{&key.PublicKey}) {
		t.Fatal("tampered body accepted")
	}
	if VerifyWechatV3Signature(ts, nonce, "dGVzdA==", body, []*rsa.PublicKey{&key.PublicKey}) {
		t.Fatal("bad sign accepted")
	}
	if VerifyWechatV3Signature(ts, nonce, sig, body, nil) {
		t.Fatal("empty pubs accepted")
	}
	if !VerifyWechatV3Signature(ts, nonce, "", body, nil) {
		t.Fatal("unsigned should pass")
	}
}

func TestShouldApplyRefundNotify(t *testing.T) {
	n := wechat.PayNotify{OutRefundNo: "RF1", RefundOK: true}
	if !wechat.ShouldApplyRefund(n) {
		t.Fatal("expected refund")
	}
	ApplyRefundNotify(n) // no DB: no-op
}
