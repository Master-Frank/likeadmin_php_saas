package cache

import "testing"

func TestPHPSerializeRoundTrip(t *testing.T) {
	in := map[string]any{
		"admin_id":    1,
		"token":       "abc",
		"role_id":     []any{1, 2},
		"expire_time": int64(123),
	}
	raw, err := phpSerialize(in)
	if err != nil {
		t.Fatal(err)
	}
	if raw == "" || raw[0] != 'a' {
		t.Fatalf("unexpected serialize: %s", raw)
	}
	out, ok := phpUnserialize(raw)
	if !ok {
		t.Fatal("unserialize failed")
	}
	m, _ := out.(map[string]any)
	if m["token"] != "abc" {
		t.Fatalf("token=%v", m["token"])
	}
}

func TestPHPUnserializeThinkPHPArray(t *testing.T) {
	raw := `a:2:{s:8:"admin_id";i:1;s:5:"token";s:3:"xyz";}`
	v, ok := phpUnserialize(raw)
	if !ok {
		t.Fatal("fail")
	}
	m := v.(map[string]any)
	if m["token"] != "xyz" {
		t.Fatalf("%v", m)
	}
}
