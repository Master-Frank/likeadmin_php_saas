package util

import "testing"

func TestDecodeJSONObject(t *testing.T) {
	v := DecodeJSON(`{"path":"/pages/index/index","name":"商城首页","type":"shop"}`)
	m, ok := v.(map[string]any)
	if !ok || m["path"] != "/pages/index/index" {
		t.Fatalf("%v", v)
	}
}

func TestToIntString(t *testing.T) {
	if ToInt("1") != 1 || ToInt("01") != 1 {
		t.Fatalf("ToInt string failed")
	}
}

func TestEncodeJSONRoundTrip(t *testing.T) {
	raw := EncodeJSON(map[string]any{"path": "/x"})
	if DecodeJSONMap(raw)["path"] != "/x" {
		t.Fatalf("%s", raw)
	}
}
