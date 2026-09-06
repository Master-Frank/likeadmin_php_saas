package biz

import (
	"encoding/json"
	"strconv"

	"likeadmin/backend/internal/bootstrap"
	"likeadmin/backend/internal/model"
	"likeadmin/backend/internal/util"

	"gorm.io/gorm"
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

func UserMoneyChangeTypes() []int {
	return []int{UMDecAdmin, UMDecRechargeRefund, UMIncAdmin, UMIncRecharge}
}

func ChangeTypeDesc(changeType int) string {
	if s, ok := UMChangeTypeDesc[strconv.Itoa(changeType)]; ok {
		return s
	}
	return ""
}

func AddAccountLog(db *gorm.DB, userID uint, tenantID uint, changeType, action int, amount, left float64, sourceSN, remark string, extra ...any) {
	if db == nil {
		db = bootstrap.DB
	}
	if db == nil {
		return
	}
	exists := func(sn string) bool {
		var n int64
		q := db.Model(&model.UserAccountLog{}).Where("sn = ?", sn)
		if tenantID > 0 {
			q = q.Where("tenant_id = ?", tenantID)
		}
		q.Count(&n)
		return n > 0
	}
	row := model.UserAccountLog{
		SN:           util.GenerateSN(exists, "20", 4),
		UserID:       userID,
		ChangeObject: UM,
		ChangeType:   changeType,
		Action:       action,
		ChangeAmount: amount,
		LeftAmount:   left,
		SourceSN:     sourceSN,
		Remark:       remark,
		TenantID:     tenantID,
		CreateTime:   util.NowUnix(),
	}
	if len(extra) > 0 {
		row.Extra = ExtraJSON(extra[0])
	}
	db.Create(&row)
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
