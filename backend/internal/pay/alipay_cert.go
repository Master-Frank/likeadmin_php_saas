package pay

import (
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"strings"
)

// attachAliCertSNs mirrors PHP EasySDK certificate-mode system params:
// app_cert_sn + alipay_root_cert_sn from AntCertificationUtil.
func attachAliCertSNs(params map[string]string, cfg AliPayCfg) {
	if params == nil || cfg.Mode != "certificate" {
		return
	}
	if sn := aliAppCertSN(cfg.PublicCert); sn != "" {
		params["app_cert_sn"] = sn
	}
	if sn := aliRootCertSN(cfg.AliRootCert); sn != "" {
		params["alipay_root_cert_sn"] = sn
	}
}

func aliAppCertSN(raw string) string {
	certs := parseCertificates(raw)
	if len(certs) == 0 {
		return ""
	}
	return aliCertSN(certs[0])
}

func aliRootCertSN(raw string) string {
	var sns []string
	for _, cert := range parseCertificates(raw) {
		if !aliRootSigOK(cert) {
			continue
		}
		if sn := aliCertSN(cert); sn != "" {
			sns = append(sns, sn)
		}
	}
	return strings.Join(sns, "_")
}

func aliRootSigOK(cert *x509.Certificate) bool {
	if cert == nil {
		return false
	}
	switch cert.SignatureAlgorithm {
	case x509.SHA1WithRSA, x509.SHA256WithRSA:
		return true
	default:
		return false
	}
}

// aliCertSN mirrors PHP AntCertificationUtil::getCertSN:
// md5(array2string(array_reverse(issuer)) . serialNumber).
func aliCertSN(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := md5.Sum([]byte(aliIssuerReversed(cert) + cert.SerialNumber.String()))
	return hex.EncodeToString(sum[:])
}

func aliIssuerReversed(cert *x509.Certificate) string {
	attrs := cert.Issuer.Names
	parts := make([]string, 0, len(attrs))
	for i := len(attrs) - 1; i >= 0; i-- {
		name := aliOIDName(attrs[i].Type.String())
		if name == "" {
			continue
		}
		val, ok := attrs[i].Value.(string)
		if !ok {
			continue
		}
		parts = append(parts, name+"="+val)
	}
	return strings.Join(parts, ",")
}

func aliOIDName(oid string) string {
	switch oid {
	case "2.5.4.3":
		return "CN"
	case "2.5.4.6":
		return "C"
	case "2.5.4.7":
		return "L"
	case "2.5.4.8":
		return "ST"
	case "2.5.4.9":
		return "STREET"
	case "2.5.4.10":
		return "O"
	case "2.5.4.11":
		return "OU"
	case "2.5.4.17":
		return "postalCode"
	case "1.2.840.113549.1.9.1":
		return "emailAddress"
	default:
		return ""
	}
}

func parseCertificates(raw string) []*x509.Certificate {
	raw = normalizePEM(raw, "CERTIFICATE")
	if raw == "" {
		return nil
	}
	var out []*x509.Certificate
	rest := []byte(raw)
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if !strings.Contains(strings.ToUpper(block.Type), "CERTIFICATE") {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		out = append(out, cert)
	}
	return out
}
