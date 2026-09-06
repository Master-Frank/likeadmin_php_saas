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
