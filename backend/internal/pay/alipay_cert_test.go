package pay

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func testAliCertPEM(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(123456789),
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{"likeadmin"},
			OrganizationalUnit: []string{"pay"},
			CommonName:         "app-cert",
		},
		Issuer: pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{"likeadmin"},
			OrganizationalUnit: []string{"pay"},
			CommonName:         "app-cert",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), key
}

func phpAliCertSN(t *testing.T, pemCert string) string {
	t.Helper()
	cmd := exec.Command("php8.3", "-r", `$ssl=openssl_x509_parse(stream_get_contents(STDIN)); $parts=[]; foreach(array_reverse($ssl["issuer"]) as $k=>$v){$parts[]=$k."=".$v;} echo md5(implode(",",$parts).$ssl["serialNumber"]);`)
	cmd.Stdin = strings.NewReader(pemCert)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("php cert sn: %v %s", err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestAliCertSNMatchesPHP(t *testing.T) {
	pemCert, _ := testAliCertPEM(t)
	want := phpAliCertSN(t, pemCert)
	got := aliAppCertSN(pemCert)
	if got == "" || got != want {
		t.Fatalf("app_cert_sn go=%s php=%s", got, want)
	}
	if aliRootCertSN(pemCert) != want {
		t.Fatalf("root sn %s want %s", aliRootCertSN(pemCert), want)
	}
}

func TestAttachAliCertSNs(t *testing.T) {
	pemCert, _ := testAliCertPEM(t)
	sn := aliAppCertSN(pemCert)
	params := map[string]string{"app_id": "2021"}
	attachAliCertSNs(params, AliPayCfg{Mode: "normal_mode", PublicCert: pemCert, AliRootCert: pemCert})
	if params["app_cert_sn"] != "" || params["alipay_root_cert_sn"] != "" {
		t.Fatalf("normal mode should omit cert sns: %+v", params)
	}
	attachAliCertSNs(params, AliPayCfg{Mode: "certificate", PublicCert: pemCert, AliRootCert: pemCert})
	if params["app_cert_sn"] != sn || params["alipay_root_cert_sn"] != sn {
		t.Fatalf("certificate sns %+v want %s", params, sn)
	}
}

func TestResolveAliPublicKeyFromCert(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "ali"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	pub := resolveAliPublicKey(AliPayCfg{Mode: "certificate", AliPublicCert: certPEM})
	if pub == nil || pub.N.Cmp(key.N) != 0 {
		t.Fatal("cert public key not resolved")
	}
	if resolveAliPublicKey(AliPayCfg{}) != nil {
		t.Fatal("empty cfg should have no key")
	}
}

func TestAliVerifyNotifyRejectsMissingKey(t *testing.T) {
	form := map[string][]string{
		"out_trade_no":    {"SN1"},
		"trade_status":    {"TRADE_SUCCESS"},
		"passback_params": {"recharge"},
	}
	if AliVerifyNotifyByTenant(0, form) {
		t.Fatal("missing public key should fail verify")
	}
}

func TestAliVerifyNotifyRejectsBadSign(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustPKIX(&key.PublicKey)}))
	cfg := AliPayCfg{AliPublicKey: pubPEM}
	pub := resolveAliPublicKey(cfg)
	params := map[string]string{"out_trade_no": "SN1", "passback_params": "recharge", "trade_status": "TRADE_SUCCESS"}
	sig, err := rsaSHA256Base64(key, aliSignContent(params))
	if err != nil {
		t.Fatal(err)
	}
	if !verifyRSA2(pub, aliSignContent(params), sig) {
		t.Fatal("good sign rejected")
	}
	if verifyRSA2(pub, aliSignContent(params), "bad") {
		t.Fatal("bad sign accepted")
	}
	form := map[string][]string{
		"out_trade_no":    {"SN1"},
		"trade_status":    {"TRADE_SUCCESS"},
		"passback_params": {"recharge"},
		"sign":            {sig},
		"sign_type":       {"RSA2"},
	}
	if !aliVerifyForm(cfg, form) {
		t.Fatal("signed notify should pass")
	}
	form["sign"] = []string{"bad"}
	if aliVerifyForm(cfg, form) {
		t.Fatal("tampered notify accepted")
	}
}

func mustPKIX(pub *rsa.PublicKey) []byte {
	b, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic(err)
	}
	return b
}
