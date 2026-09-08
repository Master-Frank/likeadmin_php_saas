package pay

import (
	"crypto/rsa"
	"encoding/json"
	"strings"
	"time"

	"likeadmin/backend/internal/cache"
)

type platCert struct {
	Serial string `json:"serial"`
	PEM    string `json:"pem"`
}

// CollectWechatPlatformPubs gathers WeChat platform public keys for V3 notify verify.
func CollectWechatPlatformPubs(prefer uint, serial string) []*rsa.PublicKey {
	seen := map[string]bool{}
	var pubs []*rsa.PublicKey
	addPEM := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" || seen[raw] {
			return
		}
		seen[raw] = true
		if pub, err := parseRSAPublicKey(raw); err == nil && pub != nil {
			pubs = append(pubs, pub)
			return
		}
		if cert, err := parseCertificate(raw); err == nil {
			if pub, ok := cert.PublicKey.(*rsa.PublicKey); ok {
				pubs = append(pubs, pub)
			}
		}
	}
	addCfg := func(tid uint) {
		cfg := WechatCfgByTenant(tid)
		addPEM(cfg.PublicKey)
		for _, pc := range cachedPlatformCerts(cfg.MchID) {
			if serial == "" || strings.EqualFold(pc.Serial, serial) {
				addPEM(pc.PEM)
			}
		}
		if cfg.MchID != "" && cfg.APIClientKey != "" && cfg.SignKey != "" {
			if serial != "" && !hasSerial(cachedPlatformCerts(cfg.MchID), serial) {
				for _, pc := range fetchWechatPlatformCerts(cfg) {
					if serial == "" || strings.EqualFold(pc.Serial, serial) {
						addPEM(pc.PEM)
					}
				}
			}
		}
	}
	if prefer > 0 {
		addCfg(prefer)
	}
	for _, tid := range WechatPayTenantIDs() {
		addCfg(tid)
	}
	addCfg(0)
	return pubs
}

func hasSerial(certs []platCert, serial string) bool {
	for _, c := range certs {
		if strings.EqualFold(c.Serial, serial) {
			return true
		}
	}
	return false
}

func cachedPlatformCerts(mchID string) []platCert {
	if mchID == "" {
		return nil
	}
	var out []platCert
	if cache.GetJSON(platCertCacheKey(mchID), &out) {
		return out
	}
	return nil
}

func platCertCacheKey(mchID string) string {
	return "wechat_plat_certs_" + mchID
}

func fetchWechatPlatformCerts(cfg WechatPayCfg) []platCert {
	if cfg.MchID == "" || cfg.APIClientKey == "" || cfg.SignKey == "" || cfg.SerialNo == "" {
		return nil
	}
	key, err := parseRSAPrivateKey(cfg.APIClientKey)
	if err != nil {
		return nil
	}
	result, err := wechatV3Get(cfg, key, "/v3/certificates")
	if err != nil || result == nil {
		return nil
	}
	raw, _ := json.Marshal(result["data"])
	var rows []struct {
		SerialNo           string `json:"serial_no"`
		EncryptCertificate struct {
			Nonce          string `json:"nonce"`
			AssociatedData string `json:"associated_data"`
			Ciphertext     string `json:"ciphertext"`
		} `json:"encrypt_certificate"`
	}
	if json.Unmarshal(raw, &rows) != nil {
		return nil
	}
	out := make([]platCert, 0, len(rows))
	for _, row := range rows {
		plain, err := aesGCMDecrypt(cfg.SignKey, row.EncryptCertificate.Nonce, row.EncryptCertificate.AssociatedData, row.EncryptCertificate.Ciphertext)
		if err != nil || len(plain) == 0 {
			continue
		}
		out = append(out, platCert{Serial: row.SerialNo, PEM: string(plain)})
	}
	if len(out) > 0 {
		cache.Set(platCertCacheKey(cfg.MchID), out, 12*time.Hour)
	}
	return out
}
