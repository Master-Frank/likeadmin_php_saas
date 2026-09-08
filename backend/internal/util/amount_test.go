package util

import "testing"

func TestFormatAmount(t *testing.T) {
	if v, ok := FormatAmount(100).(int64); !ok || v != 100 {
		t.Fatalf("int got %#v", FormatAmount(100))
	}
	if v, ok := FormatAmount(100.5).(string); !ok || v != "100.5" {
		t.Fatalf("one decimal got %#v", FormatAmount(100.5))
	}
	if v, ok := FormatAmount(100.55).(float64); !ok || v != 100.55 {
		t.Fatalf("two decimal got %#v", FormatAmount(100.55))
	}
}
