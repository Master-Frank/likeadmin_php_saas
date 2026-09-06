package pay

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"strings"

	"likeadmin/backend/internal/wechat"
)

func DecryptWechatV3(raw []byte, apiV3Key string) wechat.PayNotify {
	n := wechat.ParsePayNotify(raw, nil)
	if apiV3Key == "" || !strings.Contains(string(raw), "ciphertext") {
		return n
	}
	var env struct {
		EventType string `json:"event_type"`
		Resource  struct {
			Ciphertext     string `json:"ciphertext"`
			AssociatedData string `json:"associated_data"`
			Nonce          string `json:"nonce"`
		} `json:"resource"`
	}
	if json.Unmarshal(raw, &env) != nil || env.Resource.Ciphertext == "" {
		return n
	}
	plain, err := aesGCMDecrypt(apiV3Key, env.Resource.Nonce, env.Resource.AssociatedData, env.Resource.Ciphertext)
	if err != nil {
		return n
	}
	dec := wechat.ParsePayNotify(plain, nil)
	if env.EventType == "TRANSACTION.SUCCESS" {
		dec.Paid = true
	}
	if dec.Attach == "" {
		dec.Attach = n.Attach
	}
	return dec
}

func aesGCMDecrypt(key, nonce, aad, ciphertextB64 string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, []byte(nonce), raw, []byte(aad))
}
