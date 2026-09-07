package pay

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"strings"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/wechat"
)

func DecryptWechatV3(raw []byte, apiV3Key string) wechat.PayNotify {
	n, _ := DecryptWechatV3OK(raw, apiV3Key)
	return n
}

// DecryptWechatV3OK decrypts a V3 notify. ok is false when ciphertext is present but cannot be authenticated.
func DecryptWechatV3OK(raw []byte, apiV3Key string) (wechat.PayNotify, bool) {
	n := wechat.ParsePayNotify(raw, nil)
	if !strings.Contains(string(raw), "ciphertext") {
		return n, true
	}
	if apiV3Key == "" {
		return n, false
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
		return n, false
	}
	plain, err := aesGCMDecrypt(apiV3Key, env.Resource.Nonce, env.Resource.AssociatedData, env.Resource.Ciphertext)
	if err != nil {
		return n, false
	}
	dec := wechat.ParsePayNotify(plain, nil)
	dec.EventType = env.EventType
	if env.EventType == "TRANSACTION.SUCCESS" {
		dec.Paid = true
	}
	if env.EventType == "REFUND.SUCCESS" {
		dec.RefundOK = true
	}
	if dec.Attach == "" {
		dec.Attach = n.Attach
	}
	return dec, true
}

// DecryptWechatV3WithKeys tries each API v3 key until ciphertext authenticates.
func DecryptWechatV3WithKeys(raw []byte, keys []string) (wechat.PayNotify, bool) {
	for _, key := range keys {
		if n, ok := DecryptWechatV3OK(raw, key); ok {
			return n, true
		}
	}
	return wechat.PayNotify{}, false
}

// WechatPayTenantIDs lists tenants that have a wechat pay config row.
func WechatPayTenantIDs() []uint {
	if bootstrap.DB == nil {
		return nil
	}
	seen := map[uint]bool{}
	var out []uint
	add := func(id uint) {
		if id == 0 || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, id)
	}
	var tenants []uint
	bootstrap.DB.Model(&model.Tenant{}).Where("delete_time IS NULL").Pluck("id", &tenants)
	for _, id := range tenants {
		add(id)
	}
	var more []uint
	bootstrap.DB.Model(&model.TenantPayConfig{}).Where("pay_way = ?", WayWechat).Distinct("tenant_id").Pluck("tenant_id", &more)
	for _, id := range more {
		add(id)
	}
	return out
}

// CollectWechatSignKeys prefers the given tenant IDs, then every wechat pay config, then platform.
func CollectWechatSignKeys(prefer ...uint) []string {
	seen := map[string]bool{}
	var keys []string
	add := func(tid uint) {
		k := WechatCfgByTenant(tid).SignKey
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		keys = append(keys, k)
	}
	for _, tid := range prefer {
		add(tid)
	}
	for _, tid := range WechatPayTenantIDs() {
		add(tid)
	}
	add(0)
	return keys
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
