package util

import "testing"

func TestGenerateSN(t *testing.T) {
	seen := map[string]bool{}
	sn := GenerateSN(func(s string) bool { return seen[s] }, "", 4)
	if len(sn) != 14+4 {
		t.Fatalf("len=%d sn=%s", len(sn), sn)
	}
	seen[sn] = true
	sn2 := GenerateSN(func(s string) bool { return seen[s] }, "R", 4)
	if len(sn2) != 1+14+4 {
		t.Fatalf("prefixed len=%d", len(sn2))
	}
	if sn2[:1] != "R" {
		t.Fatalf("prefix %s", sn2)
	}
}

func TestCreateUserSN(t *testing.T) {
	seen := map[int]bool{}
	sn := CreateUserSN(func(n int) bool { return seen[n] })
	if sn < 11111111 || sn > 99999999 {
		t.Fatalf("sn range %d", sn)
	}
	s := ToString(sn)
	if len(s) != 8 {
		t.Fatalf("sn len %s", s)
	}
	for _, r := range s {
		if r < '1' || r > '9' {
			t.Fatalf("digit %q in %s", r, s)
		}
	}
	seen[sn] = true
	sn2 := CreateUserSN(func(n int) bool { return seen[n] })
	if sn2 == sn {
		t.Fatal("expected different sn")
	}
}
