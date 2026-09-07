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

	listed, _ := maskDemoValue(map[string]any{
		"tokens": []any{"ak-secret", map[string]any{"access_key": "sk", "name": "oss"}},
	}).(map[string]any)
	arr, _ := listed["tokens"].([]any)
	if len(arr) != 2 || arr[0] != "******" {
		t.Fatalf("list strings %v", listed)
	}
	inner, _ := arr[1].(map[string]any)
	if inner["access_key"] != "******" || inner["name"] != "oss" {
		t.Fatalf("list map %v", inner)
	}
}
