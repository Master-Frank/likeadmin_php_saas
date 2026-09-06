package pay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
)

func TestRSASignVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := rsaSHA256Base64(key, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyRSA2(&key.PublicKey, "hello", sig) {
		t.Fatal("verify failed")
	}
	if verifyRSA2(&key.PublicKey, "other", sig) {
		t.Fatal("bad message accepted")
	}
}

func TestParseRSAPrivateKeyPKCS1(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	raw := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	got, err := parseRSAPrivateKey(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got.N.Cmp(key.N) != 0 {
		t.Fatal("key mismatch")
	}
}

func TestAliSignContent(t *testing.T) {
	got := aliSignContent(map[string]string{"b": "2", "a": "1", "sign": "x", "empty": ""})
	if got != "a=1&b=2" {
		t.Fatalf("got %q", got)
	}
}

func TestDecryptWechatV3(t *testing.T) {
	key := "12345678901234567890123456789012"
	plain := []byte(`{"out_trade_no":"SN1234567890123456","transaction_id":"wx1","trade_state":"SUCCESS","attach":"recharge"}`)
	nonce := "123456789012"
	aad := "transaction"
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ct := gcm.Seal(nil, []byte(nonce), plain, []byte(aad))
	env := map[string]any{
		"event_type": "TRANSACTION.SUCCESS",
		"resource": map[string]any{
			"ciphertext":      base64.StdEncoding.EncodeToString(ct),
			"associated_data": aad,
			"nonce":           nonce,
		},
	}
	raw, _ := json.Marshal(env)
	n := DecryptWechatV3(raw, key)
	if !n.Paid || n.OutTradeNo != "SN1234567890123456" || n.Attach != "recharge" || n.TransactionID != "wx1" {
		t.Fatalf("%+v", n)
	}
}

func TestNormalizePEM(t *testing.T) {
	got := normalizePEM("abcd", "PUBLIC KEY")
	if got == "" || got[:10] != "-----BEGIN" {
		t.Fatalf("pem wrap failed: %s", got)
	}
}
