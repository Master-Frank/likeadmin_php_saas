package lists

import "testing"

func TestExportChunkPlan(t *testing.T) {
	got := ExportChunkPlan(0, 10, 500)
	if len(got) != 1 || got[0] != [2]int{0, 10} {
		t.Fatalf("%v", got)
	}
	got = ExportChunkPlan(100, 1200, 500)
	if len(got) != 3 || got[0] != [2]int{100, 500} || got[1] != [2]int{600, 500} || got[2] != [2]int{1100, 200} {
		t.Fatalf("%v", got)
	}
	if ExportChunkPlan(0, 0, 500) != nil {
		t.Fatal("empty")
	}
}
