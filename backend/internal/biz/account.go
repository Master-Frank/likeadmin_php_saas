package biz

import (
	"encoding/json"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"
)

const (
	UM                  = 1
	INC                 = 1
	DEC                 = 2
	UMDecAdmin          = 100
	UMDecRechargeRefund = 101
	UMIncAdmin          = 200
	UMIncRecharge       = 201
)

var UMChangeTypeDesc = map[string]string{
	"100": "平台减少余额",
	"101": "充值订单退款减少余额",
	"200": "平台增加余额",
	"201": "充值增加余额",
}

func AddAccountLog(userID uint, tenantID uint, changeType, action int, amount float64, sourceSN, remark string) {
	var user model.User
	if bootstrap.DB.First(&user, userID).Error != nil {
		return
	}
	exists := func(sn string) bool {
		var n int64
		bootstrap.DB.Model(&model.UserAccountLog{}).Where("sn = ?", sn).Count(&n)
		return n > 0
	}
	row := model.UserAccountLog{
		SN:           util.GenerateSN(exists, "", 4),
		UserID:       userID,
		ChangeObject: UM,
		ChangeType:   changeType,
		Action:       action,
		ChangeAmount: amount,
		LeftAmount:   user.UserMoney,
		SourceSN:     sourceSN,
		Remark:       remark,
		TenantID:     tenantID,
		CreateTime:   util.NowUnix(),
	}
	bootstrap.DB.Create(&row)
}

func ExtraJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}
