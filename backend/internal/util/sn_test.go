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

func TestCreateTenantSN(t *testing.T) {
	seen := map[string]bool{}
	sn := CreateTenantSN(func(s string) bool { return seen[s] })
	if len(sn) != 8 {
		t.Fatalf("len=%d sn=%s", len(sn), sn)
	}
	for _, r := range sn {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			t.Fatalf("charset %q in %s", r, sn)
		}
	}
	seen[sn] = true
	sn2 := CreateTenantSN(func(s string) bool { return seen[s] })
	if sn2 == sn {
		t.Fatal("expected different sn")
	}
}

func TestCreateTenantSNRetriesCollision(t *testing.T) {
	calls := 0
	blocked := ""
	sn := CreateTenantSN(func(s string) bool {
		calls++
		if blocked == "" {
			blocked = s
			return true
		}
		return s == blocked
	})
	if calls < 2 {
		t.Fatalf("exists called %d, want retry", calls)
	}
	if sn == blocked {
		t.Fatal("returned colliding sn")
	}
}

func TestUnixLCGRemainderPanicsAsIndex(t *testing.T) {
	// The old tenant randomSN seeded an LCG from unix seconds. By 2026 the
	// second multiply overflows int64; Go remainder stays negative.
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	n := int64(1757721600) // 2026-09-13
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		for i := 0; i < 8; i++ {
			_ = chars[int(n+int64(i*17))%len(chars)]
			n = n*1103515245 + 12345
		}
	}()
	if !panicked {
		t.Fatal("expected unix-seeded LCG remainder to panic as a string index")
	}
}
