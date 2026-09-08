package pay

import (
	"crypto/rsa"
	"strings"

	"likeadmin/backend/internal/ctxutil"

	"github.com/gin-gonic/gin"
)

// VerifyWechatV3Notify mirrors EasyWeChat Pay Server signature check.
// Missing Wechatpay-Signature keeps the decrypt-only pair/unsigned path.
// A present signature must verify against a platform public key / cert.
func VerifyWechatV3Notify(c *gin.Context, body []byte) bool {
	if c == nil {
		return true
	}
	sig := firstHeader(c, "Wechatpay-Signature")
	if sig == "" {
		return true
	}
	ts := firstHeader(c, "Wechatpay-Timestamp")
	nonce := firstHeader(c, "Wechatpay-Nonce")
	serial := firstHeader(c, "Wechatpay-Serial")
	tid := uint(0)
	if meta := ctxutil.Get(c); meta != nil {
		tid = meta.TenantID
	}
	return VerifyWechatV3Signature(ts, nonce, sig, body, CollectWechatPlatformPubs(tid, serial))
}

// VerifyWechatV3Signature checks timestamp\nnonce\nbody\n against RSA-SHA256.
func VerifyWechatV3Signature(timestamp, nonce, signature string, body []byte, pubs []*rsa.PublicKey) bool {
	if strings.TrimSpace(signature) == "" {
		return true
	}
	if timestamp == "" || nonce == "" || len(pubs) == 0 {
		return false
	}
	msg := timestamp + "\n" + nonce + "\n" + string(body) + "\n"
	for _, pub := range pubs {
		if pub != nil && verifyRSA2(pub, msg, signature) {
			return true
		}
	}
	return false
}

func firstHeader(c *gin.Context, names ...string) string {
	for _, name := range names {
		if v := strings.TrimSpace(c.GetHeader(name)); v != "" {
			return v
		}
	}
	return ""
}
