package pay

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"
)

func parseRSAPrivateKey(raw string) (*rsa.PrivateKey, error) {
	raw = normalizePEM(raw, "RSA PRIVATE KEY")
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("无效的RSA私钥")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	pkcs8, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}
	key, ok := pkcs8.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("私钥不是RSA")
	}
	return key, nil
}

func parseRSAPublicKey(raw string) (*rsa.PublicKey, error) {
	raw = normalizePEM(raw, "PUBLIC KEY")
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("无效的RSA公钥")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		pub, ok := key.(*rsa.PublicKey)
		if !ok {
			return nil, fmt.Errorf("公钥不是RSA")
		}
		return pub, nil
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, fmt.Errorf("解析公钥失败")
}

func parseCertificate(raw string) (*x509.Certificate, error) {
	raw = normalizePEM(raw, "CERTIFICATE")
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("无效的证书")
	}
	return x509.ParseCertificate(block.Bytes)
}

func certSerial(raw string) string {
	cert, err := parseCertificate(raw)
	if err != nil {
		return ""
	}
	return strings.ToUpper(fmt.Sprintf("%X", cert.SerialNumber))
}

func rsaSHA256Base64(key *rsa.PrivateKey, message string) (string, error) {
	sum := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

func verifyRSA2(pub *rsa.PublicKey, message, sign string) bool {
	raw, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false
	}
	sum := sha256.Sum256([]byte(message))
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], raw) == nil
}

func normalizePEM(raw, kind string) string {
	raw = strings.ReplaceAll(raw, `\r\n`, "\n")
	raw = strings.ReplaceAll(raw, `\n`, "\n")
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if strings.Contains(raw, "BEGIN") {
		return raw
	}
	var b strings.Builder
	b.WriteString("-----BEGIN ")
	b.WriteString(kind)
	b.WriteString("-----\n")
	for i := 0; i < len(raw); i += 64 {
		end := i + 64
		if end > len(raw) {
			end = len(raw)
		}
		b.WriteString(raw[i:end])
		b.WriteByte('\n')
	}
	b.WriteString("-----END ")
	b.WriteString(kind)
	b.WriteString("-----")
	return b.String()
}
