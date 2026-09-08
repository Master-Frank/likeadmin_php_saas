package storage

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestAliyunObjectPathEncodesLikePHP(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"uploads/a.png", "uploads/a.png"},
		{"uploads/中文 空格.png", "uploads/%E4%B8%AD%E6%96%87%20%E7%A9%BA%E6%A0%BC.png"},
		{"uploads/a%20b.png", "uploads/a%20b.png"},
	}
	for _, tc := range cases {
		if got := aliyunObject(tc.in); got != tc.want {
			t.Fatalf("aliyunObject(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestQcloudURIKeepsSlash(t *testing.T) {
	got := qcloudURI("uploads/中文.png")
	if got != "/uploads/%E4%B8%AD%E6%96%87.png" {
		t.Fatalf("qcloudURI=%q", got)
	}
}

func TestQiniuSafeB64KeepsPadding(t *testing.T) {
	// PHP base64_urlSafeEncode keeps '=' padding and maps +/ → -_.
	got := qiniuSafeB64("bucket:uploads/中文 空格.png")
	if !strings.Contains(got, "=") && len(got)%4 != 0 {
		t.Fatalf("expected padded url-safe b64: %s", got)
	}
	raw, err := base64.URLEncoding.DecodeString(got)
	if err != nil || string(raw) != "bucket:uploads/中文 空格.png" {
		t.Fatalf("roundtrip %q %v", raw, err)
	}
}
