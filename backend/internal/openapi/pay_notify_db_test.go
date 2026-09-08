package openapi

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/pay"
	"likeadmin/backend/internal/wechat"

	"github.com/gin-gonic/gin"
)

func initPayDB(t *testing.T) bool {
	t.Helper()
	if bootstrap.DB != nil {
		return true
	}
	cfg := os.Getenv("LIKEADMIN_CONFIG")
	if cfg == "" {
		cfg = "/workspace/backend/configs/config.yaml"
	}
	if err := bootstrap.Init(cfg); err != nil {
		t.Log(err)
		return false
	}
	return bootstrap.DB != nil
}

func TestMarkRechargePaidMovesMoney(t *testing.T) {
	if !initPayDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL").First(&user).Error != nil {
		t.Skip("no tenant user")
	}
	before := user.UserMoney
	beforeTotal := user.TotalRechargeAmount
	sn := "itpay" + time.Now().Format("150405.000")
	order := model.RechargeOrder{
		SN: sn, UserID: user.ID, PayWay: 2, PayStatus: 0, OrderAmount: 3.5,
		OrderTerminal: 1, TenantID: user.TenantID, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("sn = ?", sn).Delete(&model.RechargeOrder{})
		bootstrap.DB.Where("source_sn = ?", sn).Delete(&model.UserAccountLog{})
		bootstrap.DB.Model(&user).Updates(map[string]any{"user_money": before, "total_recharge_amount": beforeTotal})
	})
	if err := markRechargePaid(&order, "wx-itpay"); err != nil {
		t.Fatal(err)
	}
	var got model.RechargeOrder
	bootstrap.DB.Where("id = ?", order.ID).First(&got)
	if got.PayStatus != 1 || got.TransactionID != "wx-itpay" {
		t.Fatalf("order %+v", got)
	}
	var after model.User
	bootstrap.DB.Where("id = ?", user.ID).First(&after)
	if after.UserMoney < before+3.4 {
		t.Fatalf("money %v -> %v", before, after.UserMoney)
	}
	var logs int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("source_sn = ? AND change_type = 201", sn).Count(&logs)
	if logs != 1 {
		t.Fatalf("account logs=%d", logs)
	}
	if err := markRechargePaid(&order, "wx-itpay-2"); err != nil {
		t.Fatal(err)
	}
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("source_sn = ? AND change_type = 201", sn).Count(&logs)
	if logs != 1 {
		t.Fatalf("double pay logs=%d", logs)
	}
}

func TestMarkRechargePaidSkipsSoftDeletedUser(t *testing.T) {
	if !initPayDB(t) {
		t.Skip("no database")
	}
	var user model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL").First(&user).Error != nil {
		t.Skip("no tenant user")
	}
	now := time.Now().Unix()
	user.ID = 0
	user.SN = 80000000 + int(now%9999999)
	user.Account = "pair-soft-pay-" + time.Now().Format("150405.000")
	user.Mobile = ""
	user.UserMoney = 20
	user.TotalRechargeAmount = 20
	user.CreateTime = now
	user.UpdateTime = &now
	user.DeleteTime = &now
	if err := bootstrap.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	sn := "itsoftdel" + time.Now().Format("150405.000")
	order := model.RechargeOrder{
		SN: sn, UserID: user.ID, PayWay: 2, PayStatus: 0, OrderAmount: 5,
		OrderTerminal: 1, TenantID: 1, CreateTime: now,
	}
	if err := bootstrap.DB.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("id = ?", user.ID).Delete(&model.User{})
		bootstrap.DB.Where("id = ?", order.ID).Delete(&model.RechargeOrder{})
		bootstrap.DB.Where("source_sn = ?", sn).Delete(&model.UserAccountLog{})
	})
	if err := markRechargePaid(&order, "wx-soft"); err != nil {
		t.Fatal(err)
	}
	var after model.User
	bootstrap.DB.Where("id = ?", user.ID).First(&after)
	if after.UserMoney != 20 || after.TotalRechargeAmount != 20 {
		t.Fatalf("PHP SoftDelete User must not receive pay credit, money=%v total=%v", after.UserMoney, after.TotalRechargeAmount)
	}
	var logs int64
	bootstrap.DB.Model(&model.UserAccountLog{}).Where("source_sn = ?", sn).Count(&logs)
	if logs != 0 {
		t.Fatalf("deleted user must not get account log, logs=%d", logs)
	}
}

func TestApplyRefundNotifyMarksLog(t *testing.T) {
	if !initPayDB(t) {
		t.Skip("no database")
	}
	sn := "itrf" + time.Now().Format("150405.000")
	order := model.RechargeOrder{
		SN: sn, UserID: 1, PayWay: 2, PayStatus: 1, OrderAmount: 1,
		OrderTerminal: 1, RefundStatus: 1, TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	rec := model.RefundRecord{
		SN: sn + "r", UserID: 1, OrderID: order.ID, OrderSN: sn, OrderType: "recharge",
		OrderAmount: 1, RefundAmount: 1, RefundStatus: 0, TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&rec).Error; err != nil {
		t.Fatal(err)
	}
	lg := model.RefundLog{
		SN: sn, RecordID: rec.ID, UserID: 1, OrderAmount: 1, RefundAmount: 1,
		RefundStatus: 0, TenantID: 1, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&lg).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		bootstrap.DB.Where("id = ?", lg.ID).Delete(&model.RefundLog{})
		bootstrap.DB.Where("id = ?", rec.ID).Delete(&model.RefundRecord{})
		bootstrap.DB.Where("id = ?", order.ID).Delete(&model.RechargeOrder{})
	})
	pay.ApplyRefundNotify(wechat.PayNotify{RefundOK: true, OutRefundNo: sn, TransactionID: "wx-rf-tid"})
	var gotLog model.RefundLog
	var gotRec model.RefundRecord
	var gotOrder model.RechargeOrder
	bootstrap.DB.Where("id = ?", lg.ID).First(&gotLog)
	bootstrap.DB.Where("id = ?", rec.ID).First(&gotRec)
	bootstrap.DB.Where("id = ?", order.ID).First(&gotOrder)
	if gotLog.RefundStatus != 1 || gotRec.RefundStatus != 1 {
		t.Fatalf("log=%d rec=%d", gotLog.RefundStatus, gotRec.RefundStatus)
	}
	if gotOrder.RefundTransactionID != "wx-rf-tid" {
		t.Fatalf("refund_transaction_id=%q", gotOrder.RefundTransactionID)
	}
}

func TestHandlePayNotifyAliSignedMarksPaid(t *testing.T) {
	if !initPayDB(t) {
		t.Skip("no database")
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: mustPKIX(&key.PublicKey)}))
	var row model.TenantPayConfig
	if bootstrap.DB.Where("tenant_id = 1 AND pay_way = 3").First(&row).Error != nil {
		t.Skip("no tenant alipay config")
	}
	oldCfg := row.Config
	t.Cleanup(func() {
		bootstrap.DB.Model(&model.TenantPayConfig{}).Where("id = ?", row.ID).Update("config", oldCfg)
	})
	cfgJSON := `{"mode":"normal_mode","merchant_type":"ordinary_merchant","app_id":"app","private_key":"","ali_public_key":` + jsonQuote(pubPEM) + `}`
	if err := bootstrap.DB.Model(&model.TenantPayConfig{}).Where("id = ?", row.ID).Update("config", cfgJSON).Error; err != nil {
		t.Fatal(err)
	}

	var user model.User
	if bootstrap.DB.Where("tenant_id = 1 AND delete_time IS NULL").First(&user).Error != nil {
		t.Skip("no tenant user")
	}
	sn := "itali" + time.Now().Format("150405.000")
	order := model.RechargeOrder{
		SN: sn, UserID: user.ID, PayWay: 3, PayStatus: 0, OrderAmount: 1.2,
		OrderTerminal: 4, TenantID: user.TenantID, CreateTime: time.Now().Unix(),
	}
	if err := bootstrap.DB.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	before := user.UserMoney
	beforeTotal := user.TotalRechargeAmount
	t.Cleanup(func() {
		bootstrap.DB.Where("sn = ?", sn).Delete(&model.RechargeOrder{})
		bootstrap.DB.Where("source_sn = ?", sn).Delete(&model.UserAccountLog{})
		bootstrap.DB.Model(&user).Updates(map[string]any{"user_money": before, "total_recharge_amount": beforeTotal})
	})

	waitForm := map[string]string{
		"out_trade_no": sn, "trade_status": "WAIT_BUYER_PAY",
		"passback_params": "recharge", "trade_no": "ali-wait",
	}
	postAliNotify(t, key, waitForm)
	var waiting model.RechargeOrder
	bootstrap.DB.Where("id = ?", order.ID).First(&waiting)
	if waiting.PayStatus == 1 {
		t.Fatal("WAIT_BUYER_PAY must not mark paid")
	}

	okForm := map[string]string{
		"out_trade_no": sn, "trade_status": "TRADE_SUCCESS",
		"passback_params": "recharge", "trade_no": "ali-ok",
	}
	body := postAliNotify(t, key, okForm)
	if body != "success" {
		t.Fatalf("notify body=%s", body)
	}
	var paid model.RechargeOrder
	bootstrap.DB.Where("id = ?", order.ID).First(&paid)
	if paid.PayStatus != 1 || paid.TransactionID != "ali-ok" {
		t.Fatalf("paid %+v", paid)
	}
}

func postAliNotify(t *testing.T, key *rsa.PrivateKey, params map[string]string) string {
	t.Helper()
	sig, err := pay.SignAliNotify(key, params)
	if err != nil {
		t.Fatal(err)
	}
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	form.Set("sign", sig)
	form.Set("sign_type", "RSA2")
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/pay/aliNotify", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handlePayNotify(c)
	return w.Body.String()
}

func mustPKIX(pub *rsa.PublicKey) []byte {
	b, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic(err)
	}
	return b
}

func jsonQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
