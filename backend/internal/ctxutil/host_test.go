package ctxutil

import "testing"

func TestGetNilContext(t *testing.T) {
	if Get(nil) == nil || Get(nil).TenantID != 0 {
		t.Fatal("nil gin context must not panic")
	}
}

func TestSubDomain(t *testing.T) {
	cases := map[string]string{
		"tenant.likeadmin.test": "tenant",
		"tenant.likeadmin.test:8080": "tenant",
		"likeadmin.test": "",
		"localhost": "",
		"127.0.0.1": "",
		"127.0.0.1:8000": "",
		"a.b.c.example.com": "a.b.c",
	}
	for in, want := range cases {
		if got := SubDomain(in); got != want {
			t.Fatalf("SubDomain(%q)=%q want %q", in, got, want)
		}
	}
}
