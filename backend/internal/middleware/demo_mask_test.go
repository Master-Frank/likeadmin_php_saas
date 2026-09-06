package middleware

import "testing"

func TestMaskDemoValue(t *testing.T) {
	in := map[string]any{
		"name": "local", "access_key": "secret", "domain": "https://cdn.example",
		"nested": map[string]any{"secret_key": "abc", "icon": "logo.png"},
	}
	got, _ := maskDemoValue(in).(map[string]any)
	if got["name"] != "local" || got["access_key"] != "******" || got["domain"] != "******" {
		t.Fatalf("%v", got)
	}
	nested, _ := got["nested"].(map[string]any)
	if nested["secret_key"] != "******" || nested["icon"] != "logo.png" {
		t.Fatalf("nested=%v", nested)
	}
}
