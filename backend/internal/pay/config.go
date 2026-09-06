package pay

import (
	"encoding/json"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/ctxutil"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	WayBalance = 1
	WayWechat  = 2
	WayAli     = 3
)

type WechatPayCfg struct {
	MchID         string
	APIClientCert string
	APIClientKey  string
	SignKey       string
	SerialNo      string
}

type AliPayCfg struct {
	AppID         string
	PrivateKey    string
	AliPublicKey  string
	Mode          string
	PublicCert    string
	AliPublicCert string
	AliRootCert   string
}

func loadPayConfig(c *gin.Context, payWay int) map[string]any {
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	return loadPayConfigByTenant(tid, payWay)
}

func loadPayConfigByTenant(tenantID uint, payWay int) map[string]any {
	if bootstrap.DB == nil {
		return nil
	}
	q := bootstrap.DB.Model(&model.TenantPayConfig{}).Where("pay_way = ?", payWay)
	if tenantID > 0 {
		q = q.Where("tenant_id = ?", tenantID)
	}
	var row model.TenantPayConfig
	if q.First(&row).Error != nil {
		var plat model.PayConfig
		if bootstrap.DB.Where("pay_way = ?", payWay).First(&plat).Error != nil {
			return nil
		}
		return decodeCfg(plat.Config)
	}
	return decodeCfg(row.Config)
}

func decodeCfg(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var m map[string]any
	if json.Unmarshal([]byte(raw), &m) == nil {
		return m
	}
	return map[string]any{}
}

func WechatCfg(c *gin.Context) WechatPayCfg {
	tid := uint(0)
	if c != nil {
		tid = ctxutil.Get(c).TenantID
	}
	return WechatCfgByTenant(tid)
}

func WechatCfgByTenant(tenantID uint) WechatPayCfg {
	m := loadPayConfigByTenant(tenantID, WayWechat)
	if m == nil {
		return WechatPayCfg{}
	}
	return WechatPayCfg{
		MchID:         util.ToString(m["mch_id"]),
		APIClientCert: util.ToString(m["apiclient_cert"]),
		APIClientKey:  util.ToString(m["apiclient_key"]),
		SignKey:       firstNonEmpty(util.ToString(m["pay_sign_key"]), util.ToString(m["secret_key"])),
		SerialNo:      certSerial(util.ToString(m["apiclient_cert"])),
	}
}

func AliCfg(c *gin.Context) AliPayCfg {
	m := loadPayConfig(c, WayAli)
	if m == nil {
		return AliPayCfg{}
	}
	return AliPayCfg{
		AppID:         util.ToString(m["app_id"]),
		PrivateKey:    util.ToString(m["private_key"]),
		AliPublicKey:  util.ToString(m["ali_public_key"]),
		Mode:          util.ToString(m["mode"]),
		PublicCert:    util.ToString(m["public_cert"]),
		AliPublicCert: util.ToString(m["ali_public_cert"]),
		AliRootCert:   util.ToString(m["ali_root_cert"]),
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
