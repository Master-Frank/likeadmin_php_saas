package util

import "testing"

func TestLowerURIMatchesPHP(t *testing.T) {
	cases := map[string]string{
		"setting.storage/detail":                     "setting.storage/detail",
		"notice.smsConfig/detail":                    "notice.smsconfig/detail",
		"notice.sms_config/detail":                   "notice.smsconfig/detail",
		"channel.official_account_setting/getConfig": "channel.officialaccountsetting/getconfig",
		"channel.official_account_setting/getconfig": "channel.officialaccountsetting/getconfig",
		"channel.mnp_settings/getConfig":             "channel.mnpsettings/getconfig",
		"channel.open_setting/getConfig":             "channel.opensetting/getconfig",
		"setting.pay.pay_config/getConfig":           "setting.pay.payconfig/getconfig",
	}
	for in, want := range cases {
		if got := LowerURI(in); got != want {
			t.Fatalf("LowerURI(%q)=%q want %q", in, got, want)
		}
	}
}
