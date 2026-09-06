package util

import "encoding/json"

// DecodeJSON tries to decode a JSON object or array. Non-JSON strings are returned as-is.
func DecodeJSON(raw string) any {
	if raw == "" {
		return map[string]any{}
	}
	var obj any
	if json.Unmarshal([]byte(raw), &obj) == nil {
		return obj
	}
	return raw
}

// DecodeJSONMap decodes an object; arrays and invalid JSON become empty maps.
func DecodeJSONMap(raw string) map[string]any {
	v := DecodeJSON(raw)
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

// EncodeJSON marshals v; maps/slices stay JSON, scalars stringify.
func EncodeJSON(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			return ""
		}
		if json.Valid([]byte(t)) {
			return t
		}
		b, _ := json.Marshal(t)
		return string(b)
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}
