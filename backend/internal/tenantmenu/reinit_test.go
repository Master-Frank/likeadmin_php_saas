package tenantmenu

import "testing"

func TestReinitNilShared(t *testing.T) {
	if err := Reinit(nil, nil, 1); err != nil {
		t.Fatal(err)
	}
}
