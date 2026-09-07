package pay

import "strconv"

// FormatPaySN mirrors PHP PaymentLogic::formatOrderSn:
// $orderSn . $terminal . mb_substr((string)time(), -4)
func FormatPaySN(orderSN string, terminal int, now int64) string {
	suffix := strconv.FormatInt(now, 10)
	if n := len(suffix); n > 4 {
		suffix = suffix[n-4:]
	}
	return orderSN + strconv.Itoa(terminal) + suffix
}
