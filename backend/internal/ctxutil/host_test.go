package ctxutil

import "testing"

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
