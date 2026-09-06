package wechat

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

type PayNotify struct {
	Attach        string
	OutTradeNo    string
	TransactionID string
	Paid          bool
}

func ParsePayNotify(raw []byte, form map[string][]string) PayNotify {
	n := PayNotify{}
	if len(form) > 0 {
		get := func(k string) string {
			if vs := form[k]; len(vs) > 0 {
				return vs[0]
			}
			return ""
		}
		n.OutTradeNo = get("out_trade_no")
		n.TransactionID = firstNonEmpty(get("trade_no"), get("transaction_id"))
		n.Attach = firstNonEmpty(get("passback_params"), get("attach"))
		st := get("trade_status")
		n.Paid = st == "TRADE_SUCCESS" || st == "TRADE_FINISHED" || get("trade_state") == "SUCCESS" || get("result_code") == "SUCCESS"
		if n.OutTradeNo != "" {
			return n
		}
	}
	if len(raw) == 0 {
		return n
	}
	trim := strings.TrimSpace(string(raw))
	if strings.HasPrefix(trim, "<") {
		var x struct {
			OutTradeNo    string `xml:"out_trade_no"`
			TransactionID string `xml:"transaction_id"`
			Attach        string `xml:"attach"`
			ResultCode    string `xml:"result_code"`
			TradeState    string `xml:"trade_state"`
			ReturnCode    string `xml:"return_code"`
		}
		if xml.Unmarshal(raw, &x) == nil {
			n.OutTradeNo = x.OutTradeNo
			n.TransactionID = x.TransactionID
			n.Attach = x.Attach
			n.Paid = x.ResultCode == "SUCCESS" || x.TradeState == "SUCCESS" || (x.ReturnCode == "SUCCESS" && x.OutTradeNo != "")
		}
		return n
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) == nil {
		n.OutTradeNo = utilString(m["out_trade_no"])
		n.TransactionID = firstNonEmpty(utilString(m["transaction_id"]), utilString(m["trade_no"]))
		n.Attach = firstNonEmpty(utilString(m["attach"]), utilString(m["passback_params"]))
		st := utilString(m["trade_state"])
		if st == "" {
			st = utilString(m["event_type"])
		}
		n.Paid = st == "SUCCESS" || st == "TRANSACTION.SUCCESS" || utilString(m["trade_status"]) == "TRADE_SUCCESS"
		if res, ok := m["resource"].(map[string]any); ok && n.OutTradeNo == "" {
			n.OutTradeNo = utilString(res["out_trade_no"])
			n.TransactionID = firstNonEmpty(n.TransactionID, utilString(res["transaction_id"]))
			n.Attach = firstNonEmpty(n.Attach, utilString(res["attach"]))
		}
	}
	return n
}

func RechargeSN(outTradeNo string) string {
	rs := []rune(outTradeNo)
	if len(rs) > 18 {
		return string(rs[:18])
	}
	return outTradeNo
}

func utilString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", t), "0"), ".")
	default:
		if t == nil {
			return ""
		}
		b, _ := json.Marshal(t)
		return strings.Trim(string(b), `"`)
	}
}
