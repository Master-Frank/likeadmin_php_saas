package pay

import "testing"

func TestFormatPaySN(t *testing.T) {
	// PHP mb_substr((string)time(), -4) does not zero-pad short values.
	if got := FormatPaySN("SN1", 2, 1788803066); got != "SN123066" {
		t.Fatalf("%s", got)
	}
	if got := FormatPaySN("SN1", 1, 999); got != "SN11999" {
		t.Fatalf("short %s", got)
	}
	if got := FormatPaySN("SN1", 2, 10003); got != "SN120003" {
		t.Fatalf("mod %s", got)
	}
}
