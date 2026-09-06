package util

import "testing"

func TestValidRegisterAccount(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "请输入账号"},
		{"ab", "账号须为3-12位之间"},
		{"abcdefghijklm", "账号须为3-12位之间"},
		{"abcdef", "账号须为字母数字组合"},
		{"123456", "账号须为字母数字组合"},
		{"ab-12", "账号须为字母数字组合"},
		{"u19426", ""},
		{"pairu1", ""},
	}
	for _, c := range cases {
		if got := ValidRegisterAccount(c.in); got != c.want {
			t.Fatalf("account %q got %q want %q", c.in, got, c.want)
		}
	}
}

func TestValidRegisterPassword(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "请输入密码"},
		{"ab12", "密码须在6-25位之间"},
		{"likeadmin", "密码须为数字,字母或符号组合"},
		{"12345678", "密码须为数字,字母或符号组合"},
		{"LIKEADMIN", "密码须为数字,字母或符号组合"},
		{"!!!!!!", "密码须为数字,字母或符号组合"},
		{"likeadmin1", ""},
		{"Likeadmin1", ""},
	}
	for _, c := range cases {
		if got := ValidRegisterPassword(c.in); got != c.want {
			t.Fatalf("password %q got %q want %q", c.in, got, c.want)
		}
	}
}

func TestValidChinaMobile(t *testing.T) {
	if ValidChinaMobile("") != "请输入内容" {
		t.Fatal(ValidChinaMobile(""))
	}
	if ValidChinaMobile("123") != "手机号码格式错误" {
		t.Fatal(ValidChinaMobile("123"))
	}
	if ValidChinaMobile("13800138000") != "" {
		t.Fatal(ValidChinaMobile("13800138000"))
	}
}

func TestLoginWayAllows(t *testing.T) {
	if !LoginWayAllows([]any{"1", "2"}, 1) {
		t.Fatal("scene 1 should be allowed")
	}
	if LoginWayAllows([]any{"1", "2"}, 3) {
		t.Fatal("scene 3 should be rejected")
	}
	if LoginWayAllows([]any{"1", "2"}, 0) {
		t.Fatal("scene 0 should be rejected")
	}
}

func TestParseDateTime(t *testing.T) {
	if ParseDateTime("2024-01-02 03:04:05") == 0 {
		t.Fatal("expected unix ts")
	}
	if ParseDateTime("2024-01-02") == 0 {
		t.Fatal("expected date ts")
	}
	if ParseDateTime("") != 0 {
		t.Fatal("empty should be 0")
	}
}

func TestSexChannelMoney(t *testing.T) {
	if SexDesc(0) != "未知" || SexDesc(1) != "男" || SexDesc(2) != "女" {
		t.Fatal(SexDesc(0), SexDesc(1), SexDesc(2))
	}
	if ChannelDesc(1) != "微信小程序" || ChannelDesc(4) != "电脑PC" {
		t.Fatal(ChannelDesc(1), ChannelDesc(4))
	}
	if MoneyString(0) != "0.00" || MoneyString(1.5) != "1.50" {
		t.Fatal(MoneyString(0), MoneyString(1.5))
	}
}
