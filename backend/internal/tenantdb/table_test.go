package tenantdb

import "testing"

func TestTableWithSN(t *testing.T) {
	cases := []struct {
		name, sn, want string
	}{
		{name: "user", sn: "", want: "la_user"},
		{name: "la_user", sn: "pair2", want: "la_user_pair2"},
		{name: "recharge_order", sn: "pair2", want: "la_recharge_order"},
		{name: "la_user_account_log", sn: "pair2", want: "la_user_account_log_pair2"},
		{name: "la_user_pair2", sn: "pair2", want: "la_user_pair2"},
	}
	for _, tc := range cases {
		if got := tableWithSN(tc.name, tc.sn); got != tc.want {
			t.Fatalf("%s sn=%q: got %s want %s", tc.name, tc.sn, got, tc.want)
		}
	}
}
